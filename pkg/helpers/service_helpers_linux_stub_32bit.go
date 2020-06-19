// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build 386 arm mips mipsle

package helpers

import (
	"fmt"
	"time"
)

type ServiceDetails struct {
	Pid       string    `json:"pid"`
	Ppid      string    `json:"ppid"`
	Uids      string    `json:"uids"`         // proc/pid/status
	Gids      string    `json:"gids"`         // proc/pid/status
	Listening string    `json:"listen_socks"` // combination of /proc/pid/fds and /proc/net/(tcp|udp)
	Started   time.Time `json:"-"`
}

func GetPidDetails(pid int) (d ServiceDetails, err error) {
	err = fmt.Errorf("can't use stub function")
	return
}
