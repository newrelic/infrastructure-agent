// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sampler

import (
	"errors"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/stretchr/testify/assert"
)

type mockSampler struct {
	onStartupCalled bool
	toggleError     bool
}

var (
	eventBatch = sample.EventBatch([]sample.Event{nil, nil})
)

func (m *mockSampler) Sample() (sample.EventBatch, error) {
	if !m.onStartupCalled {
		return nil, nil
	}
	m.toggleError = !m.toggleError
	if m.toggleError {
		return nil, errors.New("error")
	} else {
		return eventBatch, nil
	}
}
func (m *mockSampler) OnStartup()              { m.onStartupCalled = true }
func (m *mockSampler) Name() string            { return "MockSampler" }
func (m *mockSampler) Interval() time.Duration { return 1 * time.Microsecond }
func (m *mockSampler) Disabled() bool          { return false }

func TestSamplerRoutine(t *testing.T) {
	// This test does not check assertions as much as it simply checks that
	// it exits without blocking.

	m := &mockSampler{}
	sampleQueue := make(chan sample.EventBatch)
	numBatches := 0
	routine := StartSamplerRoutine(m, sampleQueue)

	for {
		select {
		case sample := <-sampleQueue:
			assert.Equal(t, sample, eventBatch)
			numBatches++
			if 2 == numBatches {
				routine.Stop()
				return
			}
		}
	}
}
