// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ffHandledState_requestWasAlreadyLogged(t *testing.T) {
	var s ffHandledState

	assert.Equal(t, ffNotHandledState, s)

	assert.False(t, s.requestWasAlreadyLogged(true))
	assert.Equal(t, ffHandledEnabledState, s)

	assert.True(t, s.requestWasAlreadyLogged(true))
	assert.Equal(t, ffHandledEnabledState, s)

	assert.False(t, s.requestWasAlreadyLogged(false))
	assert.Equal(t, ffHandledEnableAndDisableState, s)

	assert.True(t, s.requestWasAlreadyLogged(false))
	assert.Equal(t, ffHandledEnableAndDisableState, s)
}
