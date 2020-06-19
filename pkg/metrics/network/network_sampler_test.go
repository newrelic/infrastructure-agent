// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/acquire"
	"github.com/stretchr/testify/assert"
)

func TestNewNetworkSampler(t *testing.T) {
	m := NewNetworkSampler(nil)

	assert.NotNil(t, m)
}

func TestNetworkSample(t *testing.T) {
	m := NewNetworkSampler(nil)

	result, err := m.Sample()

	assert.NoError(t, err)

	if len(result) > 0 {
		sample := result[0].(*NetworkSample)
		assert.Nil(t, sample.TransmitBytesPerSec)
	} else {
		t.Fatal("NetworkSampler couldn't find any networks on linux system?")
	}
}

func TestCalculateSafeDelta(t *testing.T) {
	elapsedSeconds := 1.5
	goodDelta := acquire.CalculateSafeDelta(uint64(2000), uint64(1000), elapsedSeconds)
	expectedDelta := float64(uint64(3000)-uint64(2000)) / elapsedSeconds
	assert.Equal(t, goodDelta, expectedDelta)

	negativeDelta := acquire.CalculateSafeDelta(uint64(1000), uint64(2000), elapsedSeconds)
	assert.Equal(t, negativeDelta, float64(0))

	zeroDelta := acquire.CalculateSafeDelta(uint64(2000), uint64(1000), 0)
	assert.Equal(t, zeroDelta, float64(0))
}

func TestSampleDeltas(t *testing.T) {
	t.Skip("need to refactor test")
	m := NewNetworkSampler(nil)

	// Need to pull two samples so there is a delta set between last and current
	result, err := m.Sample()
	assert.NoError(t, err)
	if len(m.lastNetStats) < 1 {
		t.Fatal("No network interfaces detected?")
	}
	for k, v := range m.lastNetStats {
		io := v
		io.BytesSent = io.BytesSent - 1000
		m.lastNetStats[k] = io
	}
	time.Sleep(1500 * time.Millisecond) // to ensure we don't divide by 0 elasped seconds
	result, err = m.Sample()
	assert.NoError(t, err)

	if len(result) > 0 {
		sample := result[0].(*NetworkSample)
		if *sample.TransmitBytesPerSec >= 100.0 {
			// Succeed
		} else {
			t.Fatalf("NetworkSampler didn't calculate delta for transmitted bytes?, %v", *sample.TransmitBytesPerSec)
		}
	} else {
		t.Fatal("NetworkSampler couldn't find any networks on linux system?")
	}
}

func BenchmarkNetwork(b *testing.B) {
	m := NewNetworkSampler(nil)
	for n := 0; n < b.N; n++ {
		_, _ = m.Sample()
	}

}
