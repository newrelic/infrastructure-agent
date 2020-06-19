// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestBasicParse(t *testing.T) {
	yamlFile := []byte(`---
labels:
  integration_group: my group
  environment: production

# we don't validate the discovery and variables,
#we just verify they are parsed
discovery:
  ttl: 1s
variables:
  myVariable:
    ttl: 1s
integrations:
  - exec: /path/to/executable
  - exec: /path/to/another/executable
`)
	config := YAML{}
	require.NoError(t, yaml.Unmarshal(yamlFile, &config))
	assert.Equal(t, "1s", config.Databind.Discovery.TTL)
	require.Contains(t, config.Databind.Variables, "myVariable")
	assert.Equal(t, "1s", config.Databind.Variables["myVariable"].TTL)
	assert.Len(t, config.Integrations, 2)
	assert.Contains(t, config.Integrations, ConfigEntry{Exec: ShlexOpt{"/path/to/executable"}})
	assert.Contains(t, config.Integrations, ConfigEntry{Exec: ShlexOpt{"/path/to/another/executable"}})
}
