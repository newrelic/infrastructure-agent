// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build slow

package obfuscate

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceObfuscatedJSONInMap(t *testing.T) {
	// newrelic agent config obfuscate --value '{"proxy_user":"myusername","proxy_password":"mypassword","redis_password":"myredispassword"}' --key jklxYusR7cxnQw
	//  "obfuscatedValue": "EUkcCjYNCg1CEB0cc01IBhUNKhABPFYOHUx9VRoZAwAgKgMzRBAPASMTSFFOFSAFEiFEFBccNVVGSR4dPRwADUcCCx0mGBgPTkJ7GAogUgcRHSEWGRgbFysRUS8="
	yaml := `
variables:
  creds:
    obfuscated:
      key: "jklxYusR7cxnQw"
      secret: "EUkcCjYNCg1CEB0cc01IBhUNKhABPFYOHUx9VRoZAwAgKgMzRBAPASMTSFFOFSAFEiFEFBccNVVGSR4dPRwADUcCCx0mGBgPTkJ7GAogUgcRHSEWGRgbFysRUS8="
`
	ctx, err := databind.LoadYAML([]byte(yaml))
	assert.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	require.NoError(t, err)

	templ := map[string]string{
		"a_key": "${creds.proxy_user}",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	// THEN a match is returned, and the JSON fields are accessible by fields and indices
	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	require.Contains(t, d, "a_key")
	assert.Equal(t, "myusername", d["a_key"])
}

func TestReplaceObfuscatedDeepJSONInMap(t *testing.T) {
	// OK:
	// newrelic agent config obfuscate --key secretPass --value '{"account":{"user":"test1","password":"test2"}}'
	//  "obfuscatedValue": "CEcCEQYbJQ8HUUkeQQcWESJDSVEHABAGVFZ8QwMSABYUHRcQcltRBxYWF0BHCS0="
	// KO:
	// newrelic agent config obfuscate --key secretPass --value '{account:{user:"test1",password:"test2"}}'
	//  "obfuscatedValue": "CAQAEQoBPhVJCAYWBgBfViQEAAdCR08CBAcjFhwBF19BBgAHJFNRDg4="
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
	require.Contains(t, d, "url")
	assert.Equal(t, "http://test1:test2@example.com/", d["url"])
}

func TestReplaceObfuscatedJSONInStruct(t *testing.T) {
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
	require.NoError(t, err)

	templ := &config.Config{
		Proxy: "http://${creds.account.user}:${creds.account.password}@example.com/",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	// THEN a match is returned, and the JSON fields are accessible by fields and indices
	require.Len(t, data, 1)
	require.IsType(t, &config.Config{}, data[0].Variables)
	d := data[0].Variables.(*config.Config)
	assert.Equal(t, "http://test1:test2@example.com/", d.Proxy)
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
