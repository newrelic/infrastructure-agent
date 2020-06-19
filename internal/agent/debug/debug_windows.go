// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package debug

import "fmt"

func init() {
	ProvideFn = func() (string, error) {
		return fmt.Sprintf("resource usage report: bytes allocated: %d", memAllocBytes()), nil
	}
}
