//go:build windows

/*
 * Copyright 2021 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package v4

import (
	"runtime"

	"github.com/shirou/gopsutil/v3/host"
)

func addOSDependantArgs(args []string) []string {
	hostInfo := getHostInfo()

	sFBLogger.
		WithField("KernelVersion", hostInfo.KernelVersion).
		WithField("PlatformFamily", hostInfo.PlatformFamily).
		WithField("PlatformVersion", hostInfo.PlatformVersion).
		Debug("windows attributes")

	if hostInfo.PlatformVersion == "something" {
		args = append(args, "one arg")
		args = append(args, "another arg")
	}
	return args
}

func getHostInfo() *host.InfoStat {
	info, err := host.Info()
	if err != nil {
		info = &host.InfoStat{
			OS: runtime.GOOS,
		}
	}
	return info
}
