// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryMonitor_SampleDarwin(t *testing.T) {
	t.Parallel()
	m := NewMemoryMonitor(false)

	sample, err := m.Sample()
	require.NoError(t, err)

	// darwin specific values
	assert.NotZero(t, sample.MemoryKernelFree)

	// linux specific values, do not send in windows
	assert.Nil(t, sample.MemoryBuffers)
}
