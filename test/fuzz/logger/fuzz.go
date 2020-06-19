// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build gofuzz
// Fuzz testing via https://github.com/dvyukov/go-fuzz

package logger

import (
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// Fuzz test for weird behaviour on agent config loading.
func Fuzz(data []byte) int {
	s := string(data)

	log.Error(data)
	log.Warn(data)
	log.Info(data)
	log.Debug(data)
	log.Trace(data)
	log.Errorf(s)                  // only one is formatted option enough
	log.WithField(s, data).Info(s) // naive approach, it'd be better to use different corpuses here

	return 0
}
