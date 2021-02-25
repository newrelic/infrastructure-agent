// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/json"
	"strconv"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
)

// Config protocol versions:
const (
	VUnsupported = Version(0)
	V1           = Version(1)
)

type Version int

// The protocol is used to register integrations without the need to use config yaml files.
// It wraps the objects used in the configuration files to define variables and integrations.
// Config protocol example.
// {
// 	"config_protocol_version": "1",
// 	"action": "register_config",
// 	"config_name": "myconfig",
// 	"config": {
// 	  "variables": {
// 		"creds": {
// 		  "vault": {
// 			"http": {
// 			  "url": "http://my.vault.host/v1/newengine/data/secret",
// 			  "headers": {
// 				"X-Vault-Token": "my-vault-token"
// 			  }
// 			}
// 		  }
// 		}
// 	  },
// 	  "integrations": [
// 		{
// 		  "name": "nri-mysql",
// 		  "interval": "15s",
// 		  "env": {
// 			"PORT": "3306",
// 			"USERNAME": "${creds.username}",
// 			"PASSWORD": "${creds.password}"
// 		  }
// 		},
// 		{
// 		  "name": "long-running-integration",
// 		  "timeout": "0"
// 		  "exec": "python /opt/integrations/my-script.py --host=127.0.0.1"
// 		}
// 	  ]
// 	}
// }

type ConfigProtocolV1 struct {
	ConfigProtocolDiscriminator
	Action     string                 `json:"action"`
	ConfigName string                 `json:"config_name"`
	Config     ConfigProtocolV1Config `json:"config"`
}

type ConfigProtocolDiscriminator struct {
	ConfigProtocolVersion string `json:"config_protocol_version"`
}

type ConfigProtocolV1Config struct {
	Databind     databind.YAMLAgentConfig `json:",inline"`
	Integrations []config.ConfigEntry     `json:"integrations"`
}

// IsConfigProtocol guesses whether a json line (coming through a previous integration response)
// belongs to config protocol payload, providing which version it belongs to in case it succeed.
func IsConfigProtocol(line []byte) (isConfigProtocol bool, ConfigProtocolVersion Version) {
	ConfigProtocolVersion = VUnsupported

	var d ConfigProtocolDiscriminator
	if err := json.Unmarshal(line, &d); err != nil {
		return
	}

	versionInt, err := strconv.Atoi(d.ConfigProtocolVersion)
	if err != nil {
		return
	}

	ConfigProtocolVersion = Version(versionInt)
	isConfigProtocol = true
	return
}

func DeserializeLine(line []byte) (r ConfigProtocolV1, err error) {
	err = json.Unmarshal(line, &r)
	return
}
