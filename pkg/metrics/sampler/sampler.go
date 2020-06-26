// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sampler

import (
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"time"
)

type Sampler interface {
	Sample() (sample.EventBatch, error)
	OnStartup()
	Name() string
	Interval() time.Duration
	Disabled() bool
}
