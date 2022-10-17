// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package snapshot

import (
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// freebsdProcess is an implementation of the process.Snapshot interface for freebsd hosts.
type freebsdProcess struct {
	// if privileged == false, some operations will be avoided: FD and IO count
	privileged bool

	stats    procStats
	process  Process
	lastCPU  CPUInfo
	lastTime time.Time

	// data that will be reused between samples of the same process
	pid     int32
	user    string
	cmdLine string
}

// needed to calculate RSS
var pageSize int64

func init() {
	pageSize = int64(os.Getpagesize())
	if pageSize <= 0 {
		pageSize = 4096 // default value
	}
}

var _ Snapshot = (*freebsdProcess)(nil) // static interface assertion

// getfreebsdProcess returns a freebsd process snapshot, trying to reuse the data from a previous snapshot of the same
// process.
func getfreebsdProcess(proc Process, privileged bool) (*freebsdProcess, error) {

	stats, err := collectProcStats(proc)
	if err != nil {
		return nil, err
	}

	return &freebsdProcess{
		privileged: privileged,
		pid:        proc.ProcessId(),
		process:    proc,
		stats:      stats,
	}, nil
}

func (pw *freebsdProcess) Pid() int32 {
	return pw.pid
}

func (pw *freebsdProcess) Username() (string, error) {
	var err error
	if pw.user == "" { // caching user
		pw.user, err = pw.process.Username()
		if err != nil {
			return "", err
		}
	}
	return pw.user, nil
}

func (pw *freebsdProcess) IOCounters() (*process.IOCountersStat, error) {
	//Not implemented in freebsd yet
	return nil, nil
}

// NumFDs returns the number of file descriptors. It returns -1 (and nil error) if the Agent does not have privileges to
// access this information.
func (pw *freebsdProcess) NumFDs() (int32, error) {
	//Not implemented in freebsd yet
	return -1, nil
}

/////////////////////////////
// Data to be derived from /proc/<pid>/stat in linux systems. In freebsd this structure will be populated
// if no error happens retrieving the information from process and will allow to cache some process vallues
// to avoid calling multiple times to same method
/////////////////////////////
type procStats struct {
	command    string
	ppid       int32
	numThreads int32
	state      string
	vmRSS      int64
	vmSize     int64
	cpu        CPUInfo
}

// collectProcStats will gather information about the process and will return procStats struct with the necessary information
// not to call process methods more than once per iteration. It will return error if any of the expected
// items returns an error.
func collectProcStats(p Process) (procStats, error) {
	var s procStats
	name, err := p.Name()
	if err != nil {
		return s, err
	}

	var ppid int32
	var parent Process
	if p.ProcessId() != 1 {
		parent, err = p.Parent()
		if err == nil {
			ppid = parent.ProcessId()
		}
	}
	numThreads, err := p.NumThreads()
	if err != nil {
		return s, err
	}
	status, err := p.Status()
	if err != nil {
		return s, err
	}
	memInfo, err := p.MemoryInfo()
	if err != nil {
		return s, err
	}
	cpuPercent, err := p.CPUPercent()
	if err != nil {
		return s, err
	}
	times, err := p.Times()
	if err != nil {
		return s, err
	}

	// unit64 to int64 conversion so there are options to lose data if rss > 9,223 PetaBytes
	rss := int64(memInfo.RSS)
	if rss > 0 {
		s.vmRSS = rss
	}
	vms := int64(memInfo.VMS)
	if vms > 0 {
		s.vmSize = vms
	}

	s.command = name
	s.ppid = ppid
	s.numThreads = numThreads
	if len(status) > 0 {
		s.state = status[0]
	}
	s.cpu = CPUInfo{
		Percent: cpuPercent,
		User:    times.User,
		System:  times.System,
	}

	return s, nil
}

func (pw *freebsdProcess) CPUTimes() (CPUInfo, error) {
	now := time.Now()

	if pw.lastTime.IsZero() {
		// invoked first time
		pw.lastCPU = pw.stats.cpu
		pw.lastTime = now
		return pw.stats.cpu, nil
	}

	// Calculate CPU percent from user time, system time, and last harvested cpu counters
	numcpu := runtime.NumCPU()
	delta := (now.Sub(pw.lastTime).Seconds()) * float64(numcpu)
	pw.stats.cpu.Percent = calculatePercent(pw.lastCPU, pw.stats.cpu, delta, numcpu)
	pw.lastCPU = pw.stats.cpu
	pw.lastTime = now

	return pw.stats.cpu, nil
}

func calculatePercent(t1, t2 CPUInfo, delta float64, numcpu int) float64 {
	if delta <= 0 {
		return 0
	}
	deltaProc := t2.User + t2.System - t1.User - t1.System
	overallPercent := ((deltaProc / delta) * 100) * float64(numcpu)
	return overallPercent
}

func (pw *freebsdProcess) Ppid() int32 {
	return pw.stats.ppid
}

func (pw *freebsdProcess) NumThreads() int32 {
	return pw.stats.numThreads
}

func (pw *freebsdProcess) Status() string {
	return pw.stats.state
}

func (pw *freebsdProcess) VmRSS() int64 {
	return pw.stats.vmRSS
}

func (pw *freebsdProcess) VmSize() int64 {
	return pw.stats.vmSize
}

func (pw *freebsdProcess) Command() string {
	return pw.stats.command
}

// CmdLine is taken from ps. As commands can have spaces, it's difficult parse parameters
// so no params for now
func (pw *freebsdProcess) CmdLine(withArgs bool) (string, error) {
	return pw.cmdLine, nil
}
