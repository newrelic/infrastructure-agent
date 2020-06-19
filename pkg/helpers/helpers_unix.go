// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

func HostProc(combineWith ...string) string {
	return GetEnv("HOST_PROC", "/proc", combineWith...)
}

func HostSys(combineWith ...string) string {
	return GetEnv("HOST_SYS", "/sys", combineWith...)
}

func HostEtc(combineWith ...string) string {
	return GetEnv("HOST_ETC", "/etc", combineWith...)
}

func HostVar(combineWith ...string) string {
	return GetEnv("HOST_VAR", "/var", combineWith...)
}
