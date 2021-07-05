// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package process

import (
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/process"
)

// darwinProcess is an implementation of the process.Snapshot interface for darwin hosts.
type darwinProcess struct {
	// if privileged == false, some operations will be avoided: FD and IO count
	privileged bool

	stats    procStats
	process  *process.Process
	lastCPU  CPUInfo
	lastTime time.Time

	// data that will be reused between samples of the same process
	pid     int32
	user    string
	cmdLine string
}

// needed to calculate RSS
var pageSize int64

// needed to calculate CPU times
var clockTicks int64

func init() {
	pageSize = int64(os.Getpagesize())
	if pageSize <= 0 {
		pageSize = 4096 // default value
	}

	//clockTicks = int64(cpu.CPUTick)
	//if clockTicks <= 0 {
	clockTicks = 100 // default value
	//}
}

var _ Snapshot = (*darwinProcess)(nil) // static interface assertion

// getDarwinProcess returns a darwin process snapshot, trying to reuse the data from a previous snapshot of the same
// process.
func getDarwinProcess(pid int32, privileged bool) (*darwinProcess, error) {
	var gops *process.Process
	var err error

	gops, err = process.NewProcess(pid)
	if err != nil {
		return nil, err
	}

	stats, err := readProcStat(gops)
	if err != nil {
		return nil, err
	}

	return &darwinProcess{
		privileged: privileged,
		pid:        pid,
		process:    gops,
		stats:      stats,
	}, nil
}

func (pw *darwinProcess) Pid() int32 {
	return pw.pid
}

func (pw *darwinProcess) Username() (string, error) {
	var err error
	if pw.user == "" { // caching user
		pw.user, err = pw.process.Username()
		if err != nil {
			return "", err
		}
	}
	return pw.user, nil
}

func (pw *darwinProcess) IOCounters() (*process.IOCountersStat, error) {
	if !pw.privileged {
		return nil, nil
	}
	return pw.process.IOCounters()
}

// NumFDs returns the number of file descriptors. It returns -1 (and nil error) if the Agent does not have privileges to
// access this information.
func (pw *darwinProcess) NumFDs() (int32, error) {
	if !pw.privileged {
		return -1, nil
	}

	return pw.process.NumFDs()
}

/////////////////////////////
// Data to be derived from /proc/<pid>/stat in linux systems. This structure will be populated
// if no error happens retrieving the information from process and will allow to implement
// Snapshot interface where no error control is considered in the majority of the methods
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

// readProcStat will gather information about the process and will return error if any of the expected
// items returns an error
func readProcStat(p *process.Process) (s procStats, err error) {
	name, err := p.Name()
	if err != nil {
		return
	}

	var ppid int32
	var parent *process.Process
	if p.Pid != 1 {
		parent, err = p.Parent()
		if err == nil {
			ppid = parent.Pid
		}
	}
	numThreads, err := p.NumThreads()
	if err != nil {
		return
	}
	status, err := p.Status()
	if err != nil {
		return
	}
	memInfo, err := p.MemoryInfo()
	if err != nil {
		return
	}

	s.command = name
	s.ppid = ppid
	s.numThreads = numThreads
	s.state = status
	rss := int64(memInfo.RSS) //TODO review this uint64 to int64 "conversion"
	if rss > 0 {
		s.vmRSS = rss
	}
	vms := int64(memInfo.VMS)
	if rss > 0 {
		s.vmSize = vms
	}
	cpuPercent, err := p.CPUPercent()
	if err != nil {
		return
	}
	times, err := p.Times()
	if err != nil {
		return
	}

	s.cpu = CPUInfo{
		Percent: cpuPercent,
		User:    times.User,
		System:  times.System,
	}

	return
}

func (pw *darwinProcess) CPUTimes() (CPUInfo, error) {
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

//func cpuInfoFromProcess(p *process.Process) (c CPUInfo, err error) {
//	timesStat, err := p.Times()
//	if err != nil {
//		return c, err
//	}
//	c.User = timesStat.User
//
//}

func calculatePercent(t1, t2 CPUInfo, delta float64, numcpu int) float64 {
	if delta == 0 {
		return 0
	}
	deltaProc := t2.User + t2.System - t1.User - t1.System
	overallPercent := ((deltaProc / delta) * 100) * float64(numcpu)
	return overallPercent
}

func (pw *darwinProcess) Ppid() int32 {
	return pw.stats.ppid
}

func (pw *darwinProcess) NumThreads() int32 {
	return pw.stats.numThreads
}

func (pw *darwinProcess) Status() string {
	return pw.stats.state
}

func (pw *darwinProcess) VmRSS() int64 {
	return pw.stats.vmRSS
}

func (pw *darwinProcess) VmSize() int64 {
	return pw.stats.vmSize
}

func (pw *darwinProcess) Command() string {
	return pw.stats.command
}

//////////////////////////
// Data to be derived from /proc/<pid>/cmdline in linux systems
// not supported in darwin for now
//////////////////////////

func (pw *darwinProcess) CmdLine(withArgs bool) (string, error) {
	return "", nil
}
