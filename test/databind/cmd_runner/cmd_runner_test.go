// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build databind
// +build databind

package cmd_runner

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceCommandOutputJSONInMap(t *testing.T) {
	yaml := `
variables:
  creds:
    command:
      path: "echo"
      args:
        - "{\"user\":\"username\",\"password\":\"testpassword\"}"
`
	ctx, err := databind.LoadYAML([]byte(yaml))
	assert.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	require.NoError(t, err)

	templ := map[string]string{
		"a_key": "${creds.user}",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	// THEN a match is returned, and the JSON fields are accessible by fields and indices
	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	require.Contains(t, d, "a_key")
	assert.Equal(t, "username", d["a_key"])
}

func TestReplaceCommandOutputDeepJSONInMap(t *testing.T) {
	yaml := `
variables:
  creds:
    command:
      path: "echo"
      args:
        - '{"account":{"user":"test1","password":"test2"}}'
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

	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	require.Contains(t, d, "url")
	assert.Equal(t, "http://test1:test2@example.com/", d["url"])
}

func TestReplaceCommandOutputJSONInStruct(t *testing.T) {
	yaml := `
variables:
  creds:
    command:
      path: "echo"
      args: ['{"account":{"user":"test1","password":"test2"}}']
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

	require.Len(t, data, 1)
	require.IsType(t, &config.Config{}, data[0].Variables)
	d := data[0].Variables.(*config.Config)
	assert.Equal(t, "http://test1:test2@example.com/", d.Proxy)
}

func TestReplaceCommandOutputPlainTextYAMLs(t *testing.T) {
	yaml := `
variables:
  creds:
    command:
      path: echo
      args: ['test']
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

	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	assert.Equal(t, "http://admin:test@example.com/", d["url"])
}

func TestPassthroughVars(t *testing.T) {
	yaml := `
variables:
  creds:
    command:
      path: "sh"
      # Careful with escaping characters here
      args: ["-c", "echo \\{\\\"user\\\":\\\"$DB_USER\\\",\\\"pass\\\":\\\"$DB_PASS\\\"\\}"]
      passthrough_environment: ["DB_USER", "DB_PASS"]
`
	t.Setenv("DB_USER", "usernameValue")
	t.Setenv("DB_PASS", "passwordValue")

	ctx, err := databind.LoadYAML([]byte(yaml))
	assert.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	require.NoError(t, err)

	templ := map[string]string{
		"a_key": "${creds.user}",
		"b_key": "${creds.pass}",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	require.Contains(t, d, "a_key")
	assert.Equal(t, "usernameValue", d["a_key"])
	require.Contains(t, d, "b_key")
	assert.Equal(t, "passwordValue", d["b_key"])
}

func TestDataWithTTL(t *testing.T) {
	yaml := `
variables:
  creds:
    command:
      path: "echo"
      args: ['{"ttl":"1234s","data":{"user":"test1","pass":"test2"}}']
`

	ctx, err := databind.LoadYAML([]byte(yaml))
	assert.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	require.NoError(t, err)

	templ := map[string]string{
		"a_key": "${creds.user}",
		"b_key": "${creds.pass}",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	require.Contains(t, d, "a_key")
	assert.Equal(t, "test1", d["a_key"])
	require.Contains(t, d, "b_key")
	assert.Equal(t, "test2", d["b_key"])
}

func TestDataWithTTL_NestedData(t *testing.T) {
	yaml := `
variables:
  creds:
    command:
      path: "echo"
      args: ['{"ttl":"1234s","data":{"account":{"user":"test1","pass":"test2"}}}']
`

	ctx, err := databind.LoadYAML([]byte(yaml))
	assert.NoError(t, err)
	vals, err := databind.Fetch(ctx)
	require.NoError(t, err)

	templ := map[string]string{
		"a_key": "${creds.account.user}",
		"b_key": "${creds.account.pass}",
	}
	data, err := databind.Replace(&vals, templ)
	require.NoError(t, err)

	require.Len(t, data, 1)
	require.IsType(t, map[string]string{}, data[0].Variables)
	d := data[0].Variables.(map[string]string)
	require.Contains(t, d, "a_key")
	assert.Equal(t, "test1", d["a_key"])
	require.Contains(t, d, "b_key")
	assert.Equal(t, "test2", d["b_key"])
}
