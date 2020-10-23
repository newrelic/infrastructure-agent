// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package commandapi

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
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
	successClient = func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(serializedCmds))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}
)

func TestClient_GetCommands_UnMarshalsData(t *testing.T) {
	client := NewClient("https://foo", "123", "Agent v0", successClient)

	cmds, err := client.GetCommands(entity.EmptyID)

	assert.NoError(t, err)
	assert.Equal(t, "backoff_command_channel", cmds[1].Name)
	assert.Equal(t, trimJSON(`{ "delay": 3000 }`), trimJSON(string(cmds[1].Args)))
	assert.Equal(t, "set_feature_flag", cmds[0].Name)
	assert.Equal(t, trimJSON(`{
				"category": "Infra_Agent",
				"flag": "flag1",
				"enabled": true
			}`), trimJSON(string(cmds[0].Args)))

}

func trimJSON(json string) string {
	return strings.Replace(strings.Replace(strings.Replace(json, "\n", "", -1), " ", "", -1), "\t", "", -1)
}
