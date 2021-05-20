// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixture struct {
	Payload  []byte
	ParsedV1 *v1
}

var cfgProtocolV1Example = &v1{
	Action:     "register_config",
	ConfigName: "myconfig",
}

var fixtureFoo = fixture{
	Payload: []byte(`
{
	"config_protocol_version": "1",
	"action": "register_config",
	"config_name": "myconfig",
	"config": {
		"variables": {},
		"integrations": [
			{
				"name": "nri-mysql",
				"interval": "15s"
			}
		]
	}
}
`),
	ParsedV1: cfgProtocolV1Example.withConfig(databind.YAMLAgentConfig{}, []config.ConfigEntry{
		{
			InstanceName: "nri-mysql",
			Interval:     "15s",
		},
	}),
}

func TestUnmarshall(t *testing.T) {
	r, err := GetConfigProtocolBuilder(fixtureFoo.Payload).Build()
	assert.NoError(t, err)
	assert.Equal(t, fixtureFoo.ParsedV1, r)
}

func TestGetConfigProtocolBuilder(t *testing.T) {
	type args struct {
		line []byte
	}
	tests := []struct {
		name                      string
		args                      args
		wantIsConfigProtocol      bool
		wantIsValid               bool
		wantConfigProtocolVersion int
	}{
		{
			name: "valid config request",
			args: args{
				line: []byte(`{
				"config_protocol_version": "1",
				"config_name": "config-name",
				"action": "action-name",
				"config": { "integrations": [ { "name": "nri-test"} ] } 
				}`),
			},
			wantIsConfigProtocol:      true,
			wantIsValid:               true,
			wantConfigProtocolVersion: 1,
		},
		{
			name: "invalid config request: missing required attributes",
			args: args{
				line: []byte(`{
					"config_protocol_version": "1"
					}`),
			},
			wantIsConfigProtocol: true,
			wantIsValid:          false,
		},
		{
			name: "different protocol",
			args: args{
				line: []byte(`{"command_request_version": "1"}`),
			},
			wantIsConfigProtocol: false,
			wantIsValid:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgProtocolBuilder := GetConfigProtocolBuilder(tt.args.line)
			if tt.wantIsConfigProtocol {
				require.NotNil(t, cfgProtocolBuilder)
			} else {
				assert.Nil(t, cfgProtocolBuilder)
			}
			if tt.wantIsValid {
				require.NotNil(t, cfgProtocolBuilder)
				cfgProtocol, err := cfgProtocolBuilder.Build()
				assert.Nil(t, err)
				assert.NotNil(t, t, cfgProtocol)
				assert.Equal(t, tt.wantConfigProtocolVersion, cfgProtocol.Version())
			}

		})
	}
}
