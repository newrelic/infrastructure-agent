// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics_sender

import (
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var mslog = log.WithField("component", "Metrics Sender")

type Sampler interface {
	Sample() (sample.EventBatch, error)
	OnStartup()
	Name() string
	Interval() time.Duration
	Disabled() bool
}
