// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package commandapi

import (
	"errors"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi/commandapitest"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
)

const serializedCmds = `
	{
		"return_value": [
			{
				"hash": "xyz",
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "flag1",
					"enabled": true
				}
			},
			{
				"hash": "xyz",
				"name": "backoff_command_channel",
				"arguments": {
					"delay": 3000
				}
			}
		]
	}
`

func TestClient_GetCommands_UnMarshalsData(t *testing.T) {
	httpClient := commandapitest.ClientReturns(200, serializedCmds, nil).Do
	client := NewClient("https://foo", "123", "Agent v0", httpClient)

	cmds, err := client.GetCommands(entity.EmptyID)

	assert.NoError(t, err)
	assert.Equal(t, "backoff_command_channel", cmds[1].Name)
	assert.Equal(t, commandapitest.TrimJSON(`{ "delay": 3000 }`), commandapitest.TrimJSON(string(cmds[1].Args)))
	assert.Equal(t, "set_feature_flag", cmds[0].Name)
	assert.Equal(t, commandapitest.TrimJSON(`{
				"category": "Infra_Agent",
				"flag": "flag1",
				"enabled": true
			}`), commandapitest.TrimJSON(string(cmds[0].Args)))
}

func TestClient_AckCommand(t *testing.T) {
	type testCase struct {
		name          string
		agentID       entity.ID
		cmdHash       string
		expectErr     bool
		expectPayload string
		client        *commandapitest.HttpClient
	}
	cases := []testCase{
		{
			"empty IDs",
			entity.EmptyID,
			"0",
			false,
			`{"hash":"0","name":"ack"}`,
			commandapitest.ClientReturns(200, serializedCmds, nil),
		},
		{
			"empty agent ID",
			entity.EmptyID,
			"1",
			false,
			`{"hash":"1","name":"ack"}`,
			commandapitest.ClientReturns(200, serializedCmds, nil),
		},
		{
			"empty cmd ID",
			1,
			"0",
			false,
			`{"hash":"0","name":"ack"}`,
			commandapitest.ClientReturns(200, serializedCmds, nil),
		},
		{
			"happy path",
			1,
			"1",
			false,
			`{"hash":"1","name":"ack"}`,
			commandapitest.ClientReturns(200, serializedCmds, nil),
		},
		{
			"http client returns 500",
			1,
			"1",
			true,
			`{"hash":"1","name":"ack"}`,
			commandapitest.ClientReturns(500, "", nil),
		},
		{
			"http client returns error",
			1,
			"123",
			true,
			`{"hash":"123","name":"ack"}`,
			commandapitest.ClientReturns(200, "", errors.New("foo")),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient("https://foo", "123", "Agent v0", tc.client.Do)

			err := client.AckCommand(tc.agentID, tc.cmdHash)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, commandapitest.TrimJSON(tc.expectPayload), commandapitest.TrimJSON(tc.client.ReceivedPayload))
		})
	}
}
