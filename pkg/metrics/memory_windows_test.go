// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryMonitor_SampleWindows(t *testing.T) {
	t.Parallel()
	m := NewMemoryMonitor(false)

	sample, err := m.Sample()
	require.NoError(t, err)

	// linux specific values, do not send in windows
	assert.Nil(t, sample.MemoryKernelFree)
	assert.Nil(t, sample.MemoryBuffers)
}

func TestCommitSwapSample(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                   string
		commitTotal, commitLim uint64
		wantTotal, wantUsed    float64
		wantFree               float64
	}{
		{"typical", 4812800000, 10261037056, 10261037056, 4812800000, 10261037056 - 4812800000},
		{"commit above limit does not underflow", 3000, 2000, 2000, 3000, 0},
		{"zero", 0, 0, 0, 0, 0},
	}
	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			got := commitSwapSample(tcase.commitTotal, tcase.commitLim)
			assert.InDelta(t, tcase.wantTotal, got.SwapTotal, 0)
			assert.InDelta(t, tcase.wantUsed, got.SwapUsed, 0)
			assert.InDelta(t, tcase.wantFree, got.SwapFree, 0)
		})
	}
}

func TestSwapMemory_Windows(t *testing.T) {
	t.Parallel()

	swap, err := swapMemory()
	require.NoError(t, err)
	assert.Positive(t, swap.SwapTotal)
	assert.GreaterOrEqual(t, swap.SwapTotal, swap.SwapFree)
}
