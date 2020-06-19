// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
)

// YAML stores the information from a single V4 integrations file
type YAML struct {
	Databind     databind.YAMLConfig `yaml:",inline"`
	Integrations []ConfigEntry       `yaml:"integrations"`
}
