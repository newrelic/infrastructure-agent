// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"time"
)

// discoverer that emits a timestamp variable
func main() {
	fmt.Printf(`[{"variables":{"timestamp":%s}}]`, fmt.Sprint(time.Now().UnixNano()))
}
