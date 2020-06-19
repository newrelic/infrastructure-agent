// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegisterManager_State(t *testing.T) {
	sm := NewRegisterSM()

	assert.Equal(t, RegisterHealthy, sm.State())

	sm.NextRetryWithBackoff()
	assert.Equal(t, RegisterRetryBackoff, sm.State())

	sm.NextRetryAfter(50 * time.Millisecond)
	assert.Equal(t, RegisterRetryAfter, sm.State())

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, RegisterHealthy, sm.State())
}
