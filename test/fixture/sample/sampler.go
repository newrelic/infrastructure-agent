// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import (
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

type Sampler struct {
	name          string
	samplesAmount int
	sample        sample.Event
}

func (m *Sampler) Interval() time.Duration { return 1 * time.Nanosecond }
func (m *Sampler) Name() string            { return m.name }
func (m *Sampler) OnStartup()              {}
func (m *Sampler) Disabled() bool          { return false }

func (m *Sampler) Sample() (s sample.EventBatch, err error) {
	if m.samplesAmount < 100 {
		m.samplesAmount++
		s = sample.EventBatch{
			m.sample,
		}
	}

	return
}

func NewSampler(sample sample.Event) sampler.Sampler {
	s := Sampler{
		sample: sample,
		name:   "FixtureSampler",
	}

	return &s
}
