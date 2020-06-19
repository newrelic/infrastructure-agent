// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package debug

import (
	"runtime"
)

var (
	// ProvideFn default debug.Provide function.
	ProvideFn = func() (string, error) {
		return "", nil
	}
)

// Provide provides debug info in human readable mode.
type Provide func() (string, error)

func memAllocBytes() uint64 {
	memstats := runtime.MemStats{}
	runtime.ReadMemStats(&memstats)

	return memstats.Alloc
}
