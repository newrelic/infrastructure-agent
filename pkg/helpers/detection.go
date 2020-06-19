// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

const (
	// Specific flavors of Linux
	LINUX_UNKNOWN = iota
	LINUX_DEBIAN
	LINUX_REDHAT
	LINUX_AWS_REDHAT
	LINUX_SUSE
	// Always add new enumerations to the end of this constant list

	// Major OS families
	OS_OSX
	OS_WINDOWS
	OS_LINUX
	OS_UNKNOWN

	LINUX_COREOS
)
