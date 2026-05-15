// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginDisabledErr(t *testing.T) {
	require.Error(t, PluginDisabledErr)
	assert.Equal(t, "frequency disabled plugin", PluginDisabledErr.Error())
}

func TestPluginDisabledErr_ImplementsError(_ *testing.T) {
	_ = PluginDisabledErr
}
