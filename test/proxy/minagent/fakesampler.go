// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package minagent

import (
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/sirupsen/logrus"
)

type FakeSampler struct {
	count int
}

func (f *FakeSampler) Sample() (sample.EventBatch, error) {
	s := FakeSample{}
	s["count"] = f.count
	f.count++
	s.Type("FakeSample")
	return sample.EventBatch{&s}, nil
}

func (*FakeSampler) OnStartup() {
	logrus.Info("Starting fake sampler")
}

func (*FakeSampler) Name() string {
	return "FakeSampler"
}

func (*FakeSampler) Disabled() bool {
	return false
}

func (*FakeSampler) Interval() time.Duration {
	return 100 * time.Millisecond
}

type FakeSample map[string]interface{}

func (f FakeSample) Type(eventType string) {
	f["eventType"] = eventType
}

func (f FakeSample) Entity(key entity.Key) {
	f["entityKey"] = key
}

func (f FakeSample) Timestamp(timestamp int64) {
	f["timestamp"] = timestamp
}
