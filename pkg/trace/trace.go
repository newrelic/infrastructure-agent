// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package trace

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type tracer struct {
	enabled map[Feature]struct{}
	logger  *logrus.Logger
}

// Condition lazy-load approach to evaluate condition only
type Condition func() bool

// global tracer instance, aimed to be used the same way log package is.
var global tracer

// EnableOn enables tracing capability for a set of features
// It uses the same logger instance as the agent.
func EnableOn(features []string) {
	if len(features) == 0 {
		return
	}

	global = tracer{
		enabled: make(map[Feature]struct{}),
		logger:  logrus.StandardLogger(),
	}

	for _, f := range features {
		global.enabled[Feature(f)] = struct{}{}
	}
}

// On logs data to be shown if trace for a given feature is enabled and condition is met.
func On(condition Condition, feature Feature, format string, args ...interface{}) {
	if global.logger == nil {
		return
	}

	if _, ok := global.enabled[feature]; ok && condition() {
		global.logger.Tracef(fmt.Sprintf("[%s] %s", feature, format), args...)
	}
}
