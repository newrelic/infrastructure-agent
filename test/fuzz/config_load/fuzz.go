// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build gofuzz
// Fuzz testing via https://github.com/dvyukov/go-fuzz

package integration_payload

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	configLoader "github.com/newrelic/infrastructure-agent/pkg/config/loader"
)

// Fuzz test for weird behaviour on agent config ingest.
func Fuzz(data []byte) int {
	c := config.Config{}
	meta, err := configLoader.ParseConfig(data, &c)
	// we still would like to look for no errors which might break oncoming config sanitization
	if err != nil {
		return 0
	}

	err = config.NormalizeConfig(&c, *meta)
	// discourage mutation when no error
	if err == nil {
		return -1
	}

	return 0
}
