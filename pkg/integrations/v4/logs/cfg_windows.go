//go:build windows

/*
 * Copyright 2021 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package logs

import (
	"github.com/shirou/gopsutil/v3/host"
	"regexp"
	"runtime"
	"strconv"
)

const winServer2016BuildNumber = 14393

var platformBuildNumberRegex = regexp.MustCompile(`.*Build ([0-9]+)`)

func addOSDependantConfig(fbOSConfig FBOSConfig) FBOSConfig {
	hostInfo := getHostInfo()

	cfgLogger.
		WithField("KernelVersion", hostInfo.KernelVersion).
		WithField("PlatformFamily", hostInfo.PlatformFamily).
		WithField("PlatformVersion", hostInfo.PlatformVersion).
		Debug("windows attributes")

	matches := platformBuildNumberRegex.FindStringSubmatch(hostInfo.PlatformVersion)

	if len(matches) == 2 {
		if buildNumber, err := strconv.Atoi(matches[1]); err == nil {
			if buildNumber <= winServer2016BuildNumber {
				cfgLogger.Debug("Use_ANSI flag set as 'True'")
				fbOSConfig.ForceUseANSI = true
			}
		}
	}

	return fbOSConfig
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
