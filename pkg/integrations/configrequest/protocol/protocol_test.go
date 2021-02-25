// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/stretchr/testify/assert"
)

type fixture struct {
	Payload  []byte
	ParsedV1 ConfigProtocolV1
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
	ParsedV1: ConfigProtocolV1{
		ConfigProtocolDiscriminator: ConfigProtocolDiscriminator{ConfigProtocolVersion: "1"},
		Action:                      "register_config",
		ConfigName:                  "myconfig",
		Config: ConfigProtocolV1Config{
			Integrations: []config.ConfigEntry{
				{
					InstanceName: "nri-mysql",
					Interval:     "15s",
				},
			},
		},
	},
}

func TestUnmarshall(t *testing.T) {
	r, err := DeserializeLine(fixtureFoo.Payload)
	assert.NoError(t, err)

	assert.Equal(t, fixtureFoo.ParsedV1, r)
}

func TestIsConfigProtocol(t *testing.T) {
	type args struct {
		line []byte
	}
	tests := []struct {
		name                      string
		args                      args
		wantIsConfigProtocol      bool
		wantConfigProtocolVersion Version
	}{
		{
			name: "valid",
			args: args{
				line: []byte(`{"config_protocol_version": "1"}`),
			},
			wantIsConfigProtocol:      true,
			wantConfigProtocolVersion: V1,
		},
		{
			name: "different protocol",
			args: args{
				line: []byte(`{"command_request_version": "1"}`),
			},
			wantIsConfigProtocol:      false,
			wantConfigProtocolVersion: VUnsupported,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIsConfigProtocol, gotConfigProtocolVersion := IsConfigProtocol(tt.args.line)
			if gotIsConfigProtocol != tt.wantIsConfigProtocol {
				t.Errorf("IsConfigProtocol() gotIsConfigProtocol = %v, want %v", gotIsConfigProtocol, tt.wantIsConfigProtocol)
			}
			if gotConfigProtocolVersion != tt.wantConfigProtocolVersion {
				t.Errorf("IsConfigProtocol() gotConfigProtocolVersion = %v, want %v", gotConfigProtocolVersion, tt.wantConfigProtocolVersion)
			}
		})
	}
}
