// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var hslog = log.WithComponent("HeartbeatSampler")

// HeartbeatSampler is used only when IsSecureForwardOnly configuration is true.
// The Rate of this sampler could be increased since we've set it up to 1 min but it could be way higher.
type HeartbeatSampler struct {
	count   int
	context agent.AgentContext
}

func NewHeartbeatSampler(context agent.AgentContext) *HeartbeatSampler {
	return &HeartbeatSampler{
		context: context,
	}
}

func (f *HeartbeatSampler) Sample() (sample.EventBatch, error) {
	s := HeartbeatSample{
		HeartbeatCounter: f.count,
		EventType:        "HeartbeatSample",
	}
	f.count++
	return sample.EventBatch{&s}, nil
}

func (*HeartbeatSampler) OnStartup() {
	hslog.Info("Starting HeartBeat sampler")
}

func (*HeartbeatSampler) Name() string {
	return "HeartbeatSampler"
}

func (hb *HeartbeatSampler) sampleInterval() int {
	if hb.context != nil {
		return hb.context.Config().HeartBeatSampleRate
	}
	// default to 60 seconds
	return config.DefaultHeartBeatFrequencySecs
}

func (hb *HeartbeatSampler) Interval() time.Duration {
	return time.Second * time.Duration(hb.sampleInterval())
}

func (hb *HeartbeatSampler) Disabled() bool {
	return hb.Interval() <= config.FREQ_DISABLE_SAMPLING
}

type HeartbeatSample struct {
	HeartbeatCounter int        `json:"heartBeatCounter"`
	EventType        string     `json:"eventType"`
	EntityKey        entity.Key `json:"entityKey"`
	Time             int64      `json:"timestamp"`
}

func (f HeartbeatSample) Type(eventType string) {
	f.EventType = eventType
}

func (f HeartbeatSample) Entity(key entity.Key) {
	f.EntityKey = key
}

func (f HeartbeatSample) Timestamp(timestamp int64) {
	f.Time = timestamp
}
