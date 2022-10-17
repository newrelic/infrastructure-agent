// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/shirou/gopsutil/v3/process"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type CommandRunner func(command string, stdin string, arguments ...string) (string, error)

var commandRunner CommandRunner = helpers.RunCommand

// ProcessRetrieverCached acts as a process.ProcessRetriever and retrieves a process.Process from its pid
// it uses an in-memory cache to store the information of all running processes with a short ttl enough to
// read information of all processes with just 2 calls to ps
// it uses c&p parts of code of gopsutil which was the 1st approach but makes too may system calls
type ProcessRetrieverCached struct {
	cache cache
}

func NewProcessRetrieverCached(ttl time.Duration) *ProcessRetrieverCached {
	return &ProcessRetrieverCached{cache: cache{ttl: ttl}}
}

// ProcessById returns a process.Process by pid or error if not found
func (s *ProcessRetrieverCached) ProcessById(pid int32) (Process, error) {
	procs, err := s.processesFromCache()
	if err != nil {
		return nil, err
	}
	if proc, ok := procs[pid]; ok {
		return &proc, nil
	}

	return nil, fmt.Errorf("cannot find process with pid %v", pid)
}

// processesFromCache returns all processes running. These will be retrieved and cached for cache.ttl time
func (s *ProcessRetrieverCached) processesFromCache() (map[int32]psItem, error) {
	s.cache.Lock()
	defer s.cache.Unlock()

	if s.cache.expired() {
		psBin, err := exec.LookPath("ps")
		if err != nil {
			return nil, err
		}
		// it's easier to get the thread num per process from different call
		processesThreads, err := s.getProcessThreads(psBin)
		if err != nil {
			return nil, err
		}
		// it's easier to get the thread num per process from different call
		fullCmd, err := s.getProcessFullCmd(psBin)
		if err != nil {
			return nil, err
		}
		//get all processes and inject numThreads
		items, err := s.retrieveProcesses(psBin)
		if err != nil {
			return nil, err
		}
		items = addThreadsAndCmdToPsItems(items, processesThreads, fullCmd)
		s.cache.update(items)
	}

	return s.cache.items, nil
}

func addThreadsAndCmdToPsItems(items map[int32]psItem, processesThreads map[int32]int32, processesCmd map[int32]string) map[int32]psItem {
	itemsWithAllInfo := make(map[int32]psItem)
	for pid, item := range items {
		if numThreads, ok := processesThreads[pid]; ok {
			item.numThreads = numThreads
		}
		if cmd, ok := processesCmd[pid]; ok {
			item.cmdLine = cmd
		}
		itemsWithAllInfo[pid] = item
	}
	return itemsWithAllInfo
}

func (s *ProcessRetrieverCached) retrieveProcesses(psBin string) (map[int32]psItem, error) {

	// get all processes info
	args := []string{"ax", "-c", "-o", "pid,ppid,user,state,utime,stime,etime,rss,vsize,pagein,command"}
	out, err := commandRunner(psBin, "", args...)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	items := make(map[int32]psItem)
	for _, line := range lines[1:] {
		var lineItems []string
		for _, lineItem := range strings.Split(line, " ") {
			if lineItem == "" {
				continue
			}
			lineItems = append(lineItems, strings.TrimSpace(lineItem))
		}
		if len(lineItems) > 10 {
			pid, _ := strconv.Atoi(lineItems[0])
			ppid, _ := strconv.Atoi(lineItems[1])
			user := lineItems[2]
			state := lineItems[3]
			utime := lineItems[4]
			stime := lineItems[5]
			etime := lineItems[6]
			rss, _ := strconv.ParseInt(lineItems[7], 10, 64)
			vsize, _ := strconv.ParseInt(lineItems[8], 10, 64)
			pagein, _ := strconv.ParseInt(lineItems[9], 10, 64)
			command := strings.Join(lineItems[10:], " ")

			item := psItem{
				pid:      int32(pid),
				ppid:     int32(ppid),
				username: user,
				state:    []string{convertStateToGopsutilState(state[0:1])},
				utime:    utime,
				stime:    stime,
				etime:    etime,
				rss:      rss,
				vsize:    vsize,
				pagein:   pagein,
				command:  command,
			}
			items[int32(pid)] = item
		} else {
			mplog.WithField("ps_output", out).Error("ps output is expected to have >10 columns")
		}
	}
	return items, nil
}

// convertStateToGopsutilState converts ps state to gopsutil v3 state
// C&P from https://github.com/shirou/gopsutil/blob/v3.21.11/v3/process/process.go#L575
func convertStateToGopsutilState(letter string) string {
	// Sources
	// Darwin: http://www.mywebuniversity.com/Man_Pages/Darwin/man_ps.html
	// FreeBSD: https://www.freebsd.org/cgi/man.cgi?ps
	// Linux https://man7.org/linux/man-pages/man1/ps.1.html
	// OpenBSD: https://man.openbsd.org/ps.1#state
	// Solaris: https://github.com/collectd/collectd/blob/1da3305c10c8ff9a63081284cf3d4bb0f6daffd8/src/processes.c#L2115
	switch letter {
	case "A":
		return process.Daemon
	case "D", "U":
		return process.Blocked
	case "E":
		return process.Detached
	case "I":
		return process.Idle
	case "L":
		return process.Lock
	case "O":
		return process.Orphan
	case "R":
		return process.Running
	case "S":
		return process.Sleep
	case "T", "t":
		// "t" is used by Linux to signal stopped by the debugger during tracing
		return process.Stop
	case "W":
		return process.Wait
	case "Y":
		return process.System
	case "Z":
		return process.Zombie
	default:
		return process.UnknownState
	}
}

func (s *ProcessRetrieverCached) getProcessThreads(psBin string) (map[int32]int32, error) {
	// get all processes info with threads
	args := []string{"ax", "-M", "-c"}
	out, err := commandRunner(psBin, "", args...)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	processThreads := make(map[int32]int32)
	for _, line := range lines[1:] {
		if len(line) > 0 && line[0] != ' ' {
			//we exclude main process for simplicity
			continue
		}
		for _, lineItem := range strings.Split(line, " ") {
			if lineItem == "" {
				continue
			}
			pidAsInt, err := strconv.Atoi(strings.TrimSpace(lineItem))
			if err != nil {
				mplog.Warnf("pid %v doesn't look like an int", pidAsInt)
				continue
			}
			pid := int32(pidAsInt)
			if _, ok := processThreads[pid]; !ok {
				processThreads[pid] = 1 //main process already included
			}
			processThreads[pid]++
			//we are only interested in pid so break and process next line
			break
		}
	}

	return processThreads, nil
}

// getProcessFullCmd retrieves the full process command line w/o arguments (as commands can have spaces in mac :( )
func (s *ProcessRetrieverCached) getProcessFullCmd(psBin string) (map[int32]string, error) {
	// get all processes info with threads
	args := []string{"ax", "-o", "pid,command"}
	out, err := commandRunner(psBin, "", args...)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	processThreads := make(map[int32]string)
	for _, line := range lines[1:] {
		var lineItems []string
		for _, lineItem := range strings.Split(line, " ") {
			if lineItem == "" {
				continue
			}
			lineItems = append(lineItems, strings.TrimSpace(lineItem))
		}
		if len(lineItems) > 1 {
			pidAsInt, _ := strconv.Atoi(lineItems[0])
			cmd := strings.Join(lineItems[1:], " ")
			pid := int32(pidAsInt)
			if _, ok := processThreads[pid]; !ok {
				processThreads[pid] = cmd
			}
		}
	}

	return processThreads, nil
}
