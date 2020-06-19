// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ctl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShutdownMonitorDarwin_noop(t *testing.T) {
	shutdownChan := make(chan shutdownCmd, 1)
	sm := newMonitor()
	err := sm.init()
	assert.NoError(t, err)
	sm.checkShutdownStatus(shutdownChan)
	close(shutdownChan)

	select {
	case cmd := <-shutdownChan:
		assert.True(t, cmd.noop)
	case <-time.After(time.Second):
		assert.FailNow(t, "Should not get here")
	}
}
