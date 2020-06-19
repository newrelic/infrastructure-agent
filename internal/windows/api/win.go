// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package winapi

// Error codes
const (
	ERROR_SUCCESS             = 0
	ERROR_INVALID_FUNCTION    = 1
	ERROR_FILE_NOT_FOUND      = 2
	ERROR_INVALID_PARAMETER   = 87
	ERROR_INSUFFICIENT_BUFFER = 122
	ERROR_MORE_DATA           = 234
)

// Function handle
type HANDLE uintptr
