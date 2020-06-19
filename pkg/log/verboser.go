// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package log

import (
	"time"

	"github.com/sirupsen/logrus"
)

var vlog = WithComponent("Verboser")

// DefaultVerboseMin default verbose time range in minutes.
const DefaultVerboseMin = 5

// We don't want to EnableTemporalVerbose if it's already enabled.
var sem = make(chan struct{}, 1)

// EnableTemporaryVerbose enables verbose logging for a given amount of minutes.
func EnableTemporaryVerbose() {
	if !shouldRun() {
		return
	}

	prevLvl := GetLevel()

	vlog.WithField("durationMin", DefaultVerboseMin).Info("setting temporal verbose logs")
	SetLevel(logrus.DebugLevel)

	go func() {
		defer finish()

		time.Sleep(time.Duration(DefaultVerboseMin) * time.Minute)

		SetLevel(prevLvl)
		vlog.WithField("level", prevLvl.String()).Info("Temporal verbose log end, restored previous log level")
	}()
}

// finish will be called when log level is restored.
func finish() {
	select {
	case <-sem:
	default:
	}
}

// shouldRun will check if there isn't an already running EnableTemporalVerbose call.
func shouldRun() bool {
	select {
	case sem <- struct{}{}:
		return true
	default:
		vlog.Info("Temporal verbose log already enabled")

		return false
	}
}
