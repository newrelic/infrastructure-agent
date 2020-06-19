// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package metrics

//go:generate go run $GOROOT/src/syscall/mksyscall_windows.go -output zsyscall_windows.go windows_privileges.go
