// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package helpers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"golang.org/x/sys/unix"
)

// Old method:
// Man times(2): the number of clock ticks per second can be obtained using:
// 	sysconf(_SC_CLK_TCK)
//
// New method:
// Source: https://github.com/prometheus/procfs/blob/master/proc_stat.go#L25-L40
//
// Originally, this USER_HZ value was dynamically retrieved via a sysconf call
// which required cgo. However, that caused a lot of problems regarding
// cross-compilation. Alternatives such as running a binary to determine the
// value, or trying to derive it in some other way were all problematic.  After
// much research it was determined that USER_HZ is actually hardcoded to 100 on
// all Go-supported platforms as of the time of this writing.
const clockTicksPerSec = 100

// cache boot time at init() time so that we don't have to do a syscall every
// time we have to check a process's age. Excessive syscalls are a solid way to
// get noticed
var _bootTime time.Time

// since figuring out listening pids is pretty expensive, we cache the results of `getPidListeners`
// in a global cache. Periodically, we clear this cache.
var _pidListenersCache map[string]pidCacheEntry
var _pidListenersCacheMutex *sync.RWMutex
var _pidListenersCounter int

type pidCacheEntry struct {
	dateCollected time.Time
	listeners     string
}

func initializePidListenersCache() {
	_pidListenersCacheMutex = &sync.RWMutex{}
	_pidListenersCache = map[string]pidCacheEntry{}
	_pidListenersCounter = 0

	go pidListenersCacheClearer()
}

func clearPidListenersCacheEntry(key string) {
	_pidListenersCacheMutex.Lock()
	defer _pidListenersCacheMutex.Unlock()
	delete(_pidListenersCache, key)
}

// every 10 minutes, look for entries older than an hour and clear them
func pidListenersCacheClearer() {
	ticker := time.Tick(10 * time.Minute)
	now := time.Now()
	for range ticker {
		for key, entry := range _pidListenersCache {
			if now.Sub(entry.dateCollected) >= time.Hour {
				clearPidListenersCacheEntry(key)
			}
		}
	}
}

func cachePidListenersEntry(pid int, listeners string) (err error) {
	var key string
	if key, err = getKeyForPid(pid); err != nil {
		return
	}
	_pidListenersCacheMutex.Lock()
	defer _pidListenersCacheMutex.Unlock()

	_pidListenersCache[key] = pidCacheEntry{time.Now(), listeners}
	return
}

type NoCacheEntry struct{}

func (self NoCacheEntry) Error() string { return "no cache entry found" }

func getPidListenersCache(pid int) (listeners string, err error) {
	var (
		key     string
		present bool
		entry   pidCacheEntry
	)
	if key, err = getKeyForPid(pid); err != nil {
		return
	}

	_pidListenersCacheMutex.RLock()
	defer _pidListenersCacheMutex.RUnlock()
	if entry, present = _pidListenersCache[key]; !present {
		err = NoCacheEntry{}
		return
	}

	listeners = entry.listeners

	return
}

func getKeyForPid(pid int) (key string, err error) {
	execPath := HostProc(strconv.Itoa(pid), "exe")
	if key, err = os.Readlink(execPath); err != nil {
		return
	}

	key = fmt.Sprintf("%d_%s", pid, key)
	return
}

func init() {
	var sysinfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		log.WithField("action", "service_helpers_linux init()").WithError(err).
			Error("could not figure out system boot time, required for service monitoring")
	}

	_bootTime = time.Now().Add(-time.Duration(sysinfo.Uptime) * time.Second)

	initializePidListenersCache()
}

func getStatusData(statusBytes []byte, wantedKeys []string) (data map[string]string) {
	sort.Strings(wantedKeys)
	data = make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewBuffer(statusBytes))
	for scanner.Scan() {
		statusLine := scanner.Text()
		keyParts := strings.Split(statusLine, ":")
		if len(keyParts) != 2 {
			continue
		}
		key, value := keyParts[0], keyParts[1]
		index := sort.SearchStrings(wantedKeys, key)
		if index < len(wantedKeys) && wantedKeys[index] == key {
			data[key] = value
		}
	}
	return
}

var whitespaceRegexp = regexp.MustCompile(`\s+`)

func cleanValue(in string) (out string) {
	return strings.TrimSpace(
		whitespaceRegexp.ReplaceAllString(in, " "),
	)
}

func getPidStartTime(pid int) (startTime time.Time, err error) {
	statPath := HostProc(fmt.Sprintf("%d", pid), "stat")
	var statBytes []byte
	if statBytes, err = ioutil.ReadFile(statPath); err != nil {
		return
	}
	fields := strings.Fields(string(statBytes))
	var startTimeClockTicks int
	// probably ParseUint is a better fit
	if startTimeClockTicks, err = strconv.Atoi(fields[21]); err != nil {
		return
	}
	startedAfterBootSec := startTimeClockTicks / clockTicksPerSec

	startTime = _bootTime.Add(time.Duration(startedAfterBootSec) * time.Second)
	return
}

func getPidListenersWithCache(pid int) (listeners string, err error) {
	if listeners, err = getPidListenersCache(pid); err != nil {
		listeners, err = getPidListeners(pid)
		cachePidListenersEntry(pid, listeners)
	}

	return
}

// the only way to find all the listening sockets for a host is to iterate all the
// file descriptors it has, find ones that are sockets, and then match those against
func getPidListeners(pid int) (string, error) {
	_pidListenersCounter++
	fdRoot := HostProc(strconv.Itoa(pid), "fd")
	var (
		symlinks []os.FileInfo
		err      error
	)

	if symlinks, err = ioutil.ReadDir(fdRoot); err != nil {
		return "", err
	}

	sockets := []os.FileInfo{}
	socketInodes := map[string]bool{}
	listenerList := []string{}
	var fd os.FileInfo
	for _, symlink := range symlinks {
		fd, err = os.Stat(filepath.Join(fdRoot, symlink.Name()))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		statResult := fd.Sys().(*syscall.Stat_t)
		if statResult.Mode&unix.S_IFSOCK != 0 {
			sockets = append(sockets, fd)
			socketInodes[strconv.FormatUint(statResult.Ino, 10)] = true
		}
	}
	if len(sockets) > 0 {
		for _, protocol := range []string{"tcp", "udp"} {
			listenerList, err = updateListeners(listenerList, protocol, socketInodes)
			if err != nil {
				return "", err
			}
		}
	}

	sort.Strings(listenerList)
	var listenersBuf []byte
	if listenersBuf, err = json.Marshal(listenerList); err != nil {
		return "", err
	}

	return string(listenersBuf), nil
}

var protocolConnStates = map[string]string{
	"tcp": "0A",
	"udp": "07",
}

func updateListeners(listeners []string, protocol string, socketInodes map[string]bool) ([]string, error) {
	var socksBuf []byte
	var err error

	fileName := HostProc(fmt.Sprintf("/net/%s", protocol))
	if socksBuf, err = ioutil.ReadFile(fileName); err != nil {
		return listeners, err
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(socksBuf))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 9 {
			// only listening sockets that belong to this process
			if fields[3] == protocolConnStates[protocol] && socketInodes[fields[9]] {
				var (
					listenPort uint64
					listenAddr string
				)
				listenRaw := strings.Split(fields[1], ":")
				if listenPort, err = strconv.ParseUint(listenRaw[1], 16, 64); err != nil {
					return listeners, err
				}
				ipParts := make([]string, 4)
				ipParts[0], ipParts[1], ipParts[2], ipParts[3] = listenRaw[0][0:2], listenRaw[0][2:4], listenRaw[0][4:6], listenRaw[0][6:8]
				for i, u := range ipParts {
					var p uint64
					if p, err = strconv.ParseUint(u, 16, 64); err != nil {
						return listeners, err
					}
					ipParts[i] = strconv.FormatUint(p, 10)
				}
				listenAddr = strings.Join(ipParts, ".")
				listeners = append(listeners, fmt.Sprintf("%s:%s:%d", strings.ToUpper(protocol), listenAddr, listenPort))
			}
		} else {
			return listeners, fmt.Errorf("'%s' file does not have the expected structure. Content size: %d. Line fields: %s", fileName, len(socksBuf), fields)
		}
	}
	return listeners, nil
}
