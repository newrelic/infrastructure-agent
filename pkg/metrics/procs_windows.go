// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package metrics

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/acquire"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	"github.com/StackExchange/wmi"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var (
	modKernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = modKernel32.NewProc("GetSystemTimes")
	modpsapi                 = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo = modpsapi.NewProc("GetProcessMemoryInfo")
	processNames             = make(map[string]bool)
	allowedListProcessing    = false
	svchostService           = regexp.MustCompile(`svchost.exe\s+-k\s+(\w+)`)
	// https://docs.microsoft.com/en-us/windows/desktop/api/winbase/nf-winbase-getprocessiocounters
	getProcessIoCounters = modkernel32.NewProc("GetProcessIoCounters")
	// https://docs.microsoft.com/en-us/windows/desktop/api/winbase/nf-winbase-queryfullprocessimagenamew
	queryFullProcessImageName = modkernel32.NewProc("QueryFullProcessImageNameW")
	containerNotRunningErrs   = map[string]struct{}{}
)

const (
	STILL_ACTIVE    = 259
	PROCESS_UNKNOWN = "na"
	PROCESS_UP      = "up"
	PROCESS_DOWN    = "down"
	SVCHOST_NAME    = "svchost.exe"

	// Process access rights: https://docs.microsoft.com/en-us/windows/desktop/procthread/process-security-and-access-rights
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

type SystemTimes struct {
	IdleTime   int64
	KernelTime int64
	UserTime   int64
}

// https://docs.microsoft.com/en-gb/windows/desktop/api/winnt/ns-winnt-_io_counters
type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type MemoryInfoStat struct {
	RSS  uint64 `json:"rss"`  // bytes
	VMS  uint64 `json:"vms"`  // bytes
	Swap uint64 `json:"swap"` // bytes
}

// abstracts the acquisition of process path
type processPathProvider func(_ syscall.Handle) (*string, error)

func (st *SystemTimes) Sub(other *SystemTimes) *SystemTimes {
	var result SystemTimes

	if other != nil {
		result.IdleTime = st.IdleTime - other.IdleTime
		result.KernelTime = st.KernelTime - other.KernelTime
		result.UserTime = st.UserTime - other.UserTime
	}

	return &result
}

func (ip *InternalProcessInterrogator) FillFromStatus(sample *ProcessSample) error {
	return nil
}

//Memory
func getMemoryInfo(pid int32) (*MemoryInfoStat, error) {
	ret := &MemoryInfoStat{}

	c, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(c)

	var mem PROCESS_MEMORY_COUNTERS
	r1, _, e1 := syscall.Syscall(procGetProcessMemoryInfo.Addr(), 3, uintptr(c), uintptr(unsafe.Pointer(&mem)), uintptr(unsafe.Sizeof(mem)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}

	if err != nil {
		return nil, err
	}

	ret.RSS = uint64(mem.WorkingSetSize)
	ret.VMS = uint64(mem.PagefileUsage)

	return ret, nil
}

// Status
func getStatus(pid int32) (string, error) {
	c, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return PROCESS_UNKNOWN, err
	}
	defer syscall.CloseHandle(c)

	var exitCode uint32
	if err = syscall.GetExitCodeProcess(c, &exitCode); err != nil {
		return PROCESS_UNKNOWN, err
	}

	if exitCode != STILL_ACTIVE {
		return PROCESS_DOWN, nil
	}

	return PROCESS_UP, nil
}

// Username
func getProcessUsername(pid int32) (string, error) {
	c, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(c)

	var t syscall.Token
	err = syscall.OpenProcessToken(c, syscall.TOKEN_QUERY, &t)
	if err != nil {
		return "", err
	}
	defer t.Close()

	tu, err := t.GetTokenUser()
	if err != nil {
		return "", err
	}
	n := uint32(64)
	dn := uint32(64)
	var accType uint32
	for {
		b := make([]uint16, n)
		db := make([]uint16, dn)
		e := syscall.LookupAccountSid(nil, tu.User.Sid, &b[0], &n, &db[0], &dn, &accType)
		if e == nil {
			return syscall.UTF16ToString(b), nil
		}
		if e != syscall.ERROR_INSUFFICIENT_BUFFER {
			return "", e
		}
		if n <= uint32(len(b)) {
			return "", e
		}
	}
}

// Process Times
func getProcessTimes(pid int32) (*SystemTimes, error) {
	var pTime syscall.Rusage

	c, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(c)

	// https://msdn.microsoft.com/en-us/library/windows/desktop/ms683223%28v=vs.85%29.aspx?f=255&MSPPError=-2147217396
	// Kernel and UserTime return a syscall.Filetime representing number of 100-ns units executed across
	// all threads of the process (cumulatively). So could be using more than one core!
	if err := syscall.GetProcessTimes(c,
		&pTime.CreationTime,
		&pTime.ExitTime,
		&pTime.KernelTime,
		&pTime.UserTime); err != nil {
		return nil, err
	}

	processTime := &SystemTimes{
		KernelTime: fileTimeToInt64(pTime.KernelTime),
		UserTime:   fileTimeToInt64(pTime.UserTime),
	}

	pslog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"processKernelTime":    processTime.KernelTime,
			"processUserTime":      processTime.UserTime,
			"rawProcessKernelTime": pTime.KernelTime,
			"rawProcessUserTime":   pTime.UserTime,
		}
	}).Debug("Raw process numbers.")

	return processTime, nil
}

func fileTimeToInt64(ft syscall.Filetime) int64 {
	// Formula used in gopsutil:
	//
	// LOT := float64(0.0000001)
	// HIT := (LOT * 4294967296.0)
	// return int64((HIT * float64(ft.HighDateTime)) + (LOT * float64(ft.LowDateTime)))

	// MSFT Conversion: https://support.microsoft.com/en-us/help/188768/info-working-with-the-filetime-structure
	return int64((uint64(ft.HighDateTime) << 32) + (uint64(ft.LowDateTime)))
}

func fileTimeToTime(filetime syscall.Filetime) time.Time {
	return time.Unix(0, filetime.Nanoseconds())
}

// Per MSDN: On multiprocessor systems, these are the sums of the values across all processors.
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724400(v=vs.85).aspx
// Should give same results as gopsutil cpu.Times() for windows. Note system time
// is just the portion of kernel time that doesn't include idle, but we'll call it kernel outside of here.
//
// Check: Time returned is the number of 100-ns units since Windows Epoch (Jan 1, 1601)?
// or just a counter of units of time
func getSystemTimes() (*SystemTimes, error) {
	var idleTime, kernelTime, userTime syscall.Filetime

	res, _, err1 := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)))
	if res != 1 {
		return nil, err1
	}

	idle := fileTimeToInt64(idleTime)
	user := fileTimeToInt64(userTime)
	kernel := fileTimeToInt64(kernelTime)
	system := (kernel - idle)

	times := &SystemTimes{
		IdleTime:   idle,
		KernelTime: system,
		UserTime:   user,
	}

	pslog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"systemKernelTime":    times.KernelTime,
			"systemUserTime":      times.UserTime,
			"systemIdleTime":      times.IdleTime,
			"rawSystemKernelTime": kernelTime,
			"rawSystemUserTime":   userTime,
			"rawSystemIdleTime":   idleTime,
		}
	}).Debug("Raw process numbers.")

	return times, nil
}

type ProcsMonitor struct {
	context              agent.AgentContext
	processInterrogator  ProcessInterrogator
	containerSampler     ContainerSampler
	procCache            map[string]*ProcessCacheEntry
	hasAlreadyRun        bool // Indicates whether the monitor has been run already (used for CPU time calculation)
	lastRun              time.Time
	currentSystemTime    *SystemTimes
	previousSystemTime   *SystemTimes
	previousProcessTimes map[string]*SystemTimes
	stopChannel          chan bool
	waitForCleanup       *sync.WaitGroup
	getAllProcs          func() ([]process.Win32_Process, error)
	getMemoryInfo        func(int32) (*MemoryInfoStat, error)
	getStatus            func(int32) (string, error)
	getUsername          func(int32) (string, error)
	getTimes             func(int32) (*SystemTimes, error)
	getCommandLine       func(uint32) (string, error)
}

func NewProcsMonitor(context agent.AgentContext) *ProcsMonitor {
	var apiVersion string
	ttlSecs := config.DefaultContainerCacheMetadataLimit
	if context != nil && context.Config() != nil {
		if len(context.Config().AllowedListProcessSample) > 0 {
			allowedListProcessing = true
			for _, processName := range context.Config().AllowedListProcessSample {
				processNames[strings.ToLower(processName)] = true
			}
		}
		ttlSecs = context.Config().ContainerMetadataCacheLimit
		apiVersion = context.Config().DockerApiVersion
	}
	return &ProcsMonitor{
		context:              context,
		procCache:            make(map[string]*ProcessCacheEntry),
		containerSampler:     NewDockerSampler(time.Duration(ttlSecs)*time.Second, apiVersion),
		previousProcessTimes: make(map[string]*SystemTimes),
		processInterrogator:  NewInternalProcessInterrogator(true),
		waitForCleanup:       &sync.WaitGroup{},
		getAllProcs:          getAllWin32Procs(getWin32APIProcessPath),
		getMemoryInfo:        getMemoryInfo,
		getStatus:            getStatus,
		getUsername:          getProcessUsername,
		getTimes:             getProcessTimes,
		getCommandLine:       getProcessCommandLineWMI,
	}
}

func (self *ProcsMonitor) calcCPUPercent(pidAndCreationDate string, currentProcessTime *SystemTimes) (float64, error) {
	var result float64
	previousProcessTime := self.previousProcessTimes[pidAndCreationDate]
	if currentProcessTime != nil && previousProcessTime != nil && self.currentSystemTime != nil && self.previousSystemTime != nil {
		processDelta := currentProcessTime.Sub(previousProcessTime)
		systemDelta := self.currentSystemTime.Sub(self.previousSystemTime)
		totalSystem := systemDelta.KernelTime + systemDelta.UserTime + systemDelta.IdleTime
		totalProcess := processDelta.KernelTime + processDelta.UserTime
		if totalSystem > 0 {
			result = float64((100.0 * totalProcess) / totalSystem)
		}
	}

	return result, nil
}

func (self *ProcsMonitor) calcCPUTimes(pidAndCreationDate string, currentProcessTime *SystemTimes) (*cpu.TimesStat, error) {
	result := &cpu.TimesStat{}
	previousProcessTime := self.previousProcessTimes[pidAndCreationDate]
	if currentProcessTime != nil && previousProcessTime != nil {
		processDelta := currentProcessTime.Sub(previousProcessTime)
		result.System = float64(processDelta.KernelTime)
		result.User = float64(processDelta.UserTime)
	}
	return result, nil
}

func (self *ProcsMonitor) calcElapsedTimeInSeconds() (elapsedSeconds float64) {
	var elapsedMs int64
	now := time.Now()
	if self.hasAlreadyRun {
		elapsedMs = (now.UnixNano() - self.lastRun.UnixNano()) / 1000000
	}
	elapsedSeconds = float64(elapsedMs) / 1000
	self.lastRun = now
	return
}

func getProcessCreationDate(handle syscall.Handle) (*time.Time, error) {
	procTimes := syscall.Rusage{}

	// https://docs.microsoft.com/en-us/windows/desktop/api/processthreadsapi/nf-processthreadsapi-getprocesstimes
	err := syscall.GetProcessTimes(handle, &procTimes.CreationTime, &procTimes.ExitTime, &procTimes.KernelTime, &procTimes.UserTime)
	if err != nil {
		return nil, err
	}

	date := fileTimeToTime(procTimes.CreationTime)
	return &date, nil
}

func getProcessIo(handle syscall.Handle) (*ioCounters, error) {
	io := &ioCounters{}

	// https://docs.microsoft.com/en-us/windows/desktop/api/winbase/nf-winbase-getprocessiocounters
	r1, _, err := getProcessIoCounters.Call(uintptr(handle), uintptr(unsafe.Pointer(io)))
	if r1 == 0 {
		return nil, err
	}

	return io, nil
}

func getWin32APIProcessPath(handle syscall.Handle) (*string, error) {
	// We are calling the Unicode version, so the string must be 16-bit
	bufferSize := uint32(syscall.MAX_PATH)
	buffer := make([]uint16, bufferSize)

	// https://docs.microsoft.com/en-us/windows/desktop/api/winbase/nf-winbase-queryfullprocessimagenamea
	r1, _, err := queryFullProcessImageName.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&bufferSize)))
	if r1 == 0 {
		return nil, err
	}

	path := syscall.UTF16ToString(buffer)
	return &path, nil
}

type win32_CommandLine struct {
	CommandLine string
}

func getProcessCommandLineWMI(processId uint32) (string, error) {
	// On Windows there is no reliable way to obtain the original command line of another process.
	// See this for more information: https://devblogs.microsoft.com/oldnewthing/20091125-00/?p=15923
	dst := []win32_CommandLine{}

	query := fmt.Sprintf("SELECT CommandLine FROM win32_process WHERE ProcessID = %d", processId)
	err := wmi.QueryNamespace(query, &dst, config.DefaultWMINamespace)
	if err != nil {
		return "", err
	}

	return dst[0].CommandLine, nil
}

func getWin32Proc(process *process.Win32_Process, path processPathProvider) error {

	// https://docs.microsoft.com/en-us/windows/desktop/api/processthreadsapi/nf-processthreadsapi-openprocess
	proc, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, process.ProcessID)
	if err != nil {
		return fmt.Errorf("cannot open process: %v", err)
	}
	defer syscall.CloseHandle(proc)

	process.CreationDate, err = getProcessCreationDate(proc)
	if err != nil {
		return fmt.Errorf("cannot retrieve timing information: %v", err)
	}

	io, err := getProcessIo(proc)
	if err != nil {
		return fmt.Errorf("cannot retrieve I/O operations: %v", err)
	}
	process.ReadOperationCount = io.ReadOperationCount
	process.ReadTransferCount = io.ReadTransferCount
	process.WriteOperationCount = io.WriteOperationCount
	process.WriteTransferCount = io.WriteTransferCount

	process.ExecutablePath, err = path(proc)
	if err != nil {
		emptyExecutablePath := ""
		process.ExecutablePath = &emptyExecutablePath
		pslog.WithError(err).WithFieldsF(func() logrus.Fields {
			return logrus.Fields{
				"name":       process.Name,
				"process_id": process.ProcessID,
			}
		}).Debug("Cannot query executable path.")
	}

	return nil
}

// We return a func for testing purpose so we can easily mock the path provider
func getAllWin32Procs(path processPathProvider) func() ([]process.Win32_Process, error) {
	return func() ([]process.Win32_Process, error) {
		var result []process.Win32_Process

		// https://docs.microsoft.com/en-us/windows/desktop/api/tlhelp32/nf-tlhelp32-createtoolhelp32snapshot
		snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
		if err != nil {
			return nil, fmt.Errorf("error creating processes snapshot: %v", err)
		}
		defer syscall.CloseHandle(snapshot)

		entry := syscall.ProcessEntry32{}
		entry.Size = uint32(unsafe.Sizeof(entry))
		// https://docs.microsoft.com/en-gb/windows/desktop/api/tlhelp32/nf-tlhelp32-process32first
		err = syscall.Process32First(snapshot, &entry)
		if err != nil {
			return nil, fmt.Errorf("error getting first element from snapshot: %v", err)
		}

		for {
			// Idle process isn't actually a process, so we can't get information from it
			if entry.ProcessID != 0 {
				proc := process.Win32_Process{
					Name:        syscall.UTF16ToString(entry.ExeFile[:]),
					ProcessID:   entry.ProcessID,
					ThreadCount: entry.Threads,
				}

				err := getWin32Proc(&proc, path)
				if err != nil {
					// Something bad happened querying this process, try next one
					pslog.WithError(err).WithFieldsF(func() logrus.Fields {
						return logrus.Fields{
							"name":       proc.Name,
							"process_id": proc.ProcessID,
						}
					}).Debug("Error retrieving process information.")
				} else {
					result = append(result, proc)
				}
			}

			// https://docs.microsoft.com/en-gb/windows/desktop/api/tlhelp32/nf-tlhelp32-process32next
			err = syscall.Process32Next(snapshot, &entry)
			if err != nil {
				break
			}
		}

		return result, nil
	}
}

func logSampleError(pid int32, winProc process.Win32_Process, err error, message string) {
	pslog.WithError(err).WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"pid":     pid,
			"winproc": winProc,
		}
	}).Debug(message)
}

func (self *ProcsMonitor) Sample() (results sample.EventBatch, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in ProcsMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	elapsedSeconds := self.calcElapsedTimeInSeconds()

	self.currentSystemTime, err = getSystemTimes()
	if err != nil {
		self.currentSystemTime = nil
		pslog.WithError(err).Error("process sampler can't determine system time")
		return
	}

	innerSamplerFunc := func() error {

		processes, err := self.getAllProcs()
		if err != nil {
			pslog.WithError(err).Error("processes can't load")
			return err
		}

		var dockerDecorator ProcessDecorator = nil
		if self.containerSampler.Enabled() {
			dockerDecorator, err = self.containerSampler.NewDecorator()
			if err != nil {
				if id := containerIDFromNotRunningErr(err); id != "" {
					if _, ok := containerNotRunningErrs[id]; !ok {
						containerNotRunningErrs[id] = struct{}{}
						pslog.WithError(err).Warn("instantiating docker sampler process decorator")
					}
				} else {
					pslog.WithError(err).Warn("instantiating docker sampler process decorator")
				}
			}
		}

		currentPids := make(map[string]bool)
		for _, winProc := range processes {

			var proc ProcessWrapper
			var lastSample *ProcessSample
			pid := int32(winProc.ProcessID)
			if pid == 0 {
				continue
			}

			pidAndCreationDate := fmt.Sprintf("%v-%v", pid, winProc.CreationDate)
			procCacheEntry := self.procCache[pidAndCreationDate]

			sample := NewProcessSample(pid)

			if procCacheEntry == nil {
				if allowedListProcessing && !processNames[strings.ToLower(winProc.Name)] {
					pslog.WithFieldsF(func() logrus.Fields {
						return logrus.Fields{
							"name": winProc.Name,
							"pid":  pid,
						}
					}).Debug("Process not in the allowed list. Skipping it.")
					continue
				}

				// We haven't encountered this process before. Create and cache
				proc, err = self.processInterrogator.NewProcess(pid)
				if err != nil {
					logSampleError(pid, winProc, err, "can't create a NewProcess")
					continue
				}
				sample.Contained = "false"
				procCacheEntry = &ProcessCacheEntry{process: proc}
				self.procCache[pidAndCreationDate] = procCacheEntry
			} else {
				proc = procCacheEntry.process
				lastSample = procCacheEntry.lastSample
			}

			if proc != nil {
				if self.Debug() {
					helpers.LogStructureDetails(pslog, winProc, "ProcWinProc", "raw", logrus.Fields{"pid": pid, "pidAndCreationDate": pidAndCreationDate})
				}
				// We saw process, so remember that for later clean up of cache
				currentPids[pidAndCreationDate] = true

				memInfo, err := self.getMemoryInfo(pid)
				if err != nil {
					logSampleError(pid, winProc, err, "can't get MemoryInfo")
					continue
				}

				if self.Debug() {
					helpers.LogStructureDetails(pslog, memInfo, "ProcMemoryInfo", "raw", logrus.Fields{"pid": pid})
				}

				sample.MemoryRSSBytes = int64(memInfo.RSS)
				sample.MemoryVMSBytes = int64(memInfo.VMS)

				// We need not report processes which are using no memory. This filters out certain kernel processes.
				if !self.DisableZeroRSSFilter() && sample.MemoryRSSBytes == 0 {
					continue
				}

				if lastSample != nil {
					// Re-use any reusable information from the last cached sample.
					sample.ParentProcessID = lastSample.ParentProcessID
					sample.CommandName = lastSample.CommandName
					sample.CmdLine = lastSample.CmdLine
					sample.User = lastSample.User
				} else {
					sample.CommandName = winProc.Name

					sample.ParentProcessID, err = proc.Ppid()
					if err != nil {
						logSampleError(pid, winProc, err, "can't get Ppid")
						continue
					}

					hasConfig := self.context != nil && self.context.Config() != nil
					stripCmdLine := (hasConfig && self.context.Config().StripCommandLine) ||
						(!hasConfig && config.DefaultStripCommandLine)

					if stripCmdLine {
						sample.CmdLine = *winProc.ExecutablePath
					} else {
						sample.CmdLine, err = self.getCommandLine(winProc.ProcessID)
						if err != nil {
							logSampleError(pid, winProc, err, "can't get command line")
							sample.CmdLine = *winProc.ExecutablePath
						}
					}

					// Need to find the key parameter for what svchost is being serviced, use that in the name
					if sample.CommandName == SVCHOST_NAME {
						matches := svchostService.FindStringSubmatch(sample.CmdLine)
						if len(matches) > 1 {
							sample.CommandName = fmt.Sprintf("%s (%s)", sample.CommandName, matches[1])
						}
					}

					sample.CmdLine = helpers.SanitizeCommandLine(sample.CmdLine)

					sample.User, err = self.getUsername(pid)
					if err != nil {
						logSampleError(pid, winProc, err, "can't get Username")
					}
				}

				// Generate a human-friendly name for this process.
				// If we know of a service for this pid, that'll be the name.
				// We can fall back to the command name if nothing else is available.
				sample.ProcessDisplayName = sample.CommandName
				if self.context != nil {
					if serviceName, ok := self.context.GetServiceForPid(int(pid)); ok {
						sample.ProcessDisplayName = serviceName
					}
				}

				// Process Status For Windows
				sample.Status, err = self.getStatus(pid)
				if err != nil {
					logSampleError(pid, winProc, err, "can't get process status")
				}

				currentProcessTime, err := self.getTimes(pid)
				if err != nil {
					logSampleError(pid, winProc, err, "can't get process times")
				} else {
					if self.Debug() {
						helpers.LogStructureDetails(pslog, currentProcessTime, "ProcGetProcessTimes", "raw", logrus.Fields{"pid": pid})
					}
					sample.CPUPercent, err = self.calcCPUPercent(pidAndCreationDate, currentProcessTime)
					if err != nil {
						logSampleError(pid, winProc, err, "can't get CPUPercent")
						goto processTime
					}

					cpuTimes, err := self.calcCPUTimes(pidAndCreationDate, currentProcessTime)
					if err != nil {
						logSampleError(pid, winProc, err, "can't get CPUTimes")
						goto processTime
					}
					// determine the proportion of the total cpu time that is user vs system
					// Note that the underlying library may not be populating all cpuTimes fields
					totalCPU := cpuTotal(cpuTimes)
					cpuUser := cpuTimes.User + cpuTimes.Nice
					cpuSystem := totalCPU - cpuUser

					if totalCPU > 0 {
						sample.CPUUserPercent = (cpuUser / totalCPU) * sample.CPUPercent
						sample.CPUSystemPercent = (cpuSystem / totalCPU) * sample.CPUPercent
					} else {
						sample.CPUUserPercent = 0
						sample.CPUSystemPercent = 0
					}
				}
			processTime:
				self.previousProcessTimes[pidAndCreationDate] = currentProcessTime

				sample.ThreadCount = int32(winProc.ThreadCount)

				ioCounters := &process.IOCountersStat{
					ReadCount:  uint64(winProc.ReadOperationCount),
					ReadBytes:  uint64(winProc.ReadTransferCount),
					WriteCount: uint64(winProc.WriteOperationCount),
					WriteBytes: uint64(winProc.WriteTransferCount),
				}
				if ioCounters != nil {
					// Delta
					if lastSample != nil && lastSample.LastIOCounters != nil {
						lastCounters := lastSample.LastIOCounters

						ioReadCountPerSecond := acquire.CalculateSafeDelta(ioCounters.ReadCount, lastCounters.ReadCount, elapsedSeconds)
						ioWriteCountPerSecond := acquire.CalculateSafeDelta(ioCounters.WriteCount, lastCounters.WriteCount, elapsedSeconds)
						ioReadBytesPerSecond := acquire.CalculateSafeDelta(ioCounters.ReadBytes, lastCounters.ReadBytes, elapsedSeconds)
						ioWriteBytesPerSecond := acquire.CalculateSafeDelta(ioCounters.WriteBytes, lastCounters.WriteBytes, elapsedSeconds)

						sample.IOReadCountPerSecond = &ioReadCountPerSecond
						sample.IOWriteCountPerSecond = &ioWriteCountPerSecond
						sample.IOReadBytesPerSecond = &ioReadBytesPerSecond
						sample.IOWriteBytesPerSecond = &ioWriteBytesPerSecond
					}
					// Cumulative
					sample.IOTotalReadCount = &ioCounters.ReadCount
					sample.IOTotalWriteCount = &ioCounters.WriteCount
					sample.IOTotalReadBytes = &ioCounters.ReadBytes
					sample.IOTotalWriteBytes = &ioCounters.WriteBytes

					sample.LastIOCounters = ioCounters
				}

				sample.Type("ProcessSample")
				if dockerDecorator != nil {
					dockerDecorator.Decorate(sample)
				}
				procCacheEntry.lastSample = sample
				results = append(results, sample)
			}
		}
		// clear the err, we are just logging and dropping such samples,
		// real errors will cause this function to return immediately
		err = nil

		// Clean up any cached process data for PIDs we don't see running anymore.
		// This is necessary in case the system starts reusing PIDs.
		for pidAndCreationDate := range self.procCache {
			if _, ok := currentPids[pidAndCreationDate]; !ok {
				delete(self.procCache, pidAndCreationDate)
				delete(self.previousProcessTimes, pidAndCreationDate)
			}
		}

		return err
	}

	if self.EnableElevatedProcessPriv() {
		err = RunWithPrivilege(SeDebugPrivilege, innerSamplerFunc)
	} else {
		err = innerSamplerFunc()
	}

	self.hasAlreadyRun = true
	self.previousSystemTime = self.currentSystemTime

	if self.Debug() {
		for _, sample := range results {
			helpers.LogStructureDetails(pslog, sample.(*ProcessSample), "ProcessSample", "final", nil)
		}
	}
	return
}

func (self *ProcsMonitor) Debug() bool {
	if self.context == nil {
		return false
	}
	return self.context.Config().Debug
}

func (self *ProcsMonitor) DisableZeroRSSFilter() bool {
	if self.context == nil {
		return false
	}
	return self.context.Config().DisableZeroRSSFilter
}

func (self *ProcsMonitor) EnableElevatedProcessPriv() bool {
	if self.context == nil {
		return false
	}
	return self.context.Config().EnableElevatedProcessPriv
}

// deprecated. Used only by the windows ProcsMonitor
func cpuTotal(t *cpu.TimesStat) float64 {
	return t.User + t.System + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal +
		t.Guest + t.GuestNice + t.Idle
}

func (self *ProcsMonitor) intervalSecs() int {
	if self.context != nil {
		return self.context.Config().MetricsProcessSampleRate
	}

	return config.FREQ_INTERVAL_FLOOR_PROCESS_METRICS
}

func (self *ProcsMonitor) Name() string {
	return "ProcessSampler"
}

func (self *ProcsMonitor) Interval() time.Duration {
	return time.Second * time.Duration(self.intervalSecs())
}

func (self *ProcsMonitor) OnStartup() {}

func (self *ProcsMonitor) Disabled() bool {
	return self.Interval() <= config.FREQ_DISABLE_SAMPLING
}

func containerIDFromNotRunningErr(err error) string {
	prefix := "Error response from daemon: Container "
	suffix := " is not running"
	msg := err.Error()
	i := strings.Index(msg, prefix)
	j := strings.Index(msg, suffix)
	if i == -1 || j == -1 {
		return ""
	}
	return msg[len(prefix):j]
}
