// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build amd64 arm64 mips64 mips64le ppc64 ppc64le s390x

package helpers

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ServiceDetails struct {
	Pid       string    `json:"pid"`
	Ppid      string    `json:"ppid"`
	Uids      string    `json:"uids"`         // proc/pid/status
	Gids      string    `json:"gids"`         // proc/pid/status
	Listening string    `json:"listen_socks"` // combination of /proc/pid/fds and /proc/net/(tcp|udp)
	Started   time.Time `json:"-"`
	// IsRealBinary string `json:"match_on_disk"` // proc/pid/exe + proc/pid/maps
	// ListeningOn  string `json:"listening_on"`  // proc/pid/fds? /proc/pid/net/???
}

func GetPidDetails(pid int) (d ServiceDetails, err error) {
	procRoot := HostProc(strconv.Itoa(pid))

	if _, err = os.Stat(procRoot); err != nil {
		return
	}

	var statusBytes []byte
	if statusBytes, err = ioutil.ReadFile(filepath.Join(procRoot, "status")); err != nil {
		return
	}

	data := getStatusData(statusBytes, []string{"Uid", "Gid", "PPid"})

	d.Pid = strconv.Itoa(pid)
	d.Ppid = cleanValue(data["PPid"])
	d.Uids = cleanValue(data["Uid"])
	d.Gids = cleanValue(data["Gid"])
	if d.Started, err = getPidStartTime(pid); err != nil {
		return
	}
	if d.Listening, err = getPidListenersWithCache(pid); err != nil {
		return
	}

	return
}
