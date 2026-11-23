//go:build windows
// +build windows

/*
 * Copyright 2023 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package logs

import (
	"regexp"
	"runtime"
	"strconv"

	"github.com/shirou/gopsutil/v3/host"
)

const (
	numMatchesExpected       = 2
	winServer2016BuildNumber = 14393
)

var platformBuildNumberRegex = regexp.MustCompile(`.*Build ([0-9]+)`)

func addOSDependantConfig(fbOSConfig *FBOSConfig) {
	hostInfo := getHostInfo()

	cfgLogger.
		WithField("KernelVersion", hostInfo.KernelVersion).
		WithField("PlatformFamily", hostInfo.PlatformFamily).
		WithField("PlatformVersion", hostInfo.PlatformVersion).
		Debug("windows attributes")

	matches := platformBuildNumberRegex.FindStringSubmatch(hostInfo.PlatformVersion)

	if len(matches) == numMatchesExpected {
		if buildNumber, err := strconv.Atoi(matches[1]); err == nil {
			if buildNumber <= winServer2016BuildNumber {
				cfgLogger.Debug("Use_ANSI flag set as 'True'")
				fbOSConfig.UseANSI = true
			}
		}
	}
}

//nolint:exhaustruct
func getHostInfo() *host.InfoStat {
	info, err := host.Info()
	if err != nil {
		info = &host.InfoStat{
			OS: runtime.GOOS,
		}
	}

	return info
}
