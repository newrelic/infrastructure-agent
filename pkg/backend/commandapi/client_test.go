// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package commandapi

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const serializedCmds = `
	{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "flag1",
					"enabled": true
				}
			},
			{
				"id": 0,
				"name": "backoff_command_channel",
				"arguments": {
					"delay": 3000
				}
			}
		]
	}
`

var (
	expectedCmds = []Command{
		{
			Name: "set_feature_flag",
			Args: FFArgs{
				Category: "Infra_Agent",
				Flag:     "flag1",
				Enabled:  true,
			},
		},
		{
			Name: "backoff_command_channel",
			Args: BackoffArgs{
				Delay: 3000,
			},
		},
	}
	successClient = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(serializedCmds))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}
)

func TestUnmarshalJSONCmd_UnkownCmd(t *testing.T) {
	serializedCmd := []byte(`{
		"id": 0,
		"name": "unknown_cmd"
	}`)

	var c Command
	err := json.Unmarshal(serializedCmd, &c)
	require.Equal(t, UnknownCmdErr, err)
}

func TestUnmarshalJSONCmd_SetFFCmd(t *testing.T) {
	serializedCmd := []byte(`{
		"id": 0,
		"name": "set_feature_flag",
		"arguments": {
			"category": "Infra_Agent",
			"flag": "flag1",
			"enabled": true
		}
	}`)

	var c Command
	err := json.Unmarshal(serializedCmd, &c)
	require.NoError(t, err)
	assert.Equal(t, "set_feature_flag", c.Name)
	require.IsType(t, FFArgs{}, c.Args)
	ffArgs := c.Args.(FFArgs)
	assert.Equal(t, "Infra_Agent", ffArgs.Category)
	assert.True(t, ffArgs.Enabled)
	assert.Equal(t, "flag1", ffArgs.Flag)
}

func TestUnmarshalJSONCmd_BackoffCmd(t *testing.T) {
	serializedCmd := []byte(`{
		"id": 0,
		"name": "backoff_command_channel",
		"arguments": {
			"delay": 3000
		}
	}`)

	var c Command
	err := json.Unmarshal(serializedCmd, &c)
	require.NoError(t, err)
	assert.Equal(t, "backoff_command_channel", c.Name)
	require.IsType(t, BackoffArgs{}, c.Args)
	assert.Equal(t, 3000, c.Args.(BackoffArgs).Delay)
}

func TestClient_GetCommands_UnMarshalsData(t *testing.T) {
	client := NewClient("https://foo", "123", "Agent v0", successClient)

	cmds, err := client.GetCommands(entity.EmptyID)

	assert.NoError(t, err)
	assert.Equal(t, expectedCmds, cmds)
}
