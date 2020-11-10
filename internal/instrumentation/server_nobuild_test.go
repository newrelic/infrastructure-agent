// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build  !go1.13

package instrumentation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpentelemetryServer(t *testing.T) {
	exporter, err := NewOpentelemetryServer()
	require.NoError(t, err)
	require.NotNil(t, exporter)
	assert.IsType(t, &noop{}, exporter)
}
