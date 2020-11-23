// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package distro

func GetDistro() string {
	return "windows"
}

func IsCentos5() bool {
	return false
}
