// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin && harvest
// +build darwin,harvest

package harvest

import (
	"testing"
)

func TestHostDisk(t *testing.T) {
	t.Skipf("Skipped until storage sampler is supported for darwin")
}
