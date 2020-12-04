// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build slow

package obfuscate

import (
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReplaceObfuscatedJSONYAMLs(t *testing.T) {
	// For the encoded secret {"account":{"user":"test1","password":"test2"}}
	yaml := `
variables:
  creds:
    obfuscated:
      key: "secretPass"
      secret: "CEcCEQYbJQ8HUUkeQQcWESJDSVEHABAGVFZ8QwMSABYUHRcQcltRBxYWF0BHCS0="
`
	ctx, err := databind.LoadYAML([]byte(yaml))
	assert.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	_ = vals
	require.NoError(t, err)
	templ := map[string]string{
		"url": "http://${creds.account.user}:${creds.account.password}@example.com/",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	// THEN a match is returned, and the JSON fields are accessible by fields and indices
	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	assert.Equal(t, "http://test1:test2@example.com/", d["url"])
}

func TestReplaceObfuscatedPlainTextYAMLs(t *testing.T) {
	// For the encoded secret 'test'
	yaml := `
variables:
  creds:
    obfuscated:
      key: secretPass
      secret: BwAQBg==
`

	ctx, err := databind.LoadYAML([]byte(yaml))
	assert.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	_ = vals
	require.NoError(t, err)
	templ := map[string]string{
		"url": "http://admin:${creds}@example.com/",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	// THEN a match is returned, and the JSON fields are accessible by fields and indices
	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	assert.Equal(t, "http://admin:test@example.com/", d["url"])
}
