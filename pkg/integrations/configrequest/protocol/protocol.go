// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/json"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track/ctx"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
)

type discriminator struct {
	Version string `json:"config_protocol_version"`
}

type Builder interface {
	Build() (ConfigProtocol, error)
}

type builder struct {
	content []byte
	fn      func() ConfigProtocol
}

func (builder *builder) Build() (ConfigProtocol, error) {
	var cfgProtocol = builder.fn()
	err := json.Unmarshal(builder.content, cfgProtocol)
	return cfgProtocol, err
}

var defaultBuilderFn = func() ConfigProtocol {
	return &v1{}
}

var cfgProtocolVersions = map[string]func() ConfigProtocol{
	"1": func() ConfigProtocol { return &v1{} },
}

type ConfigProtocol interface {
	Name() string
	Version() int
	BuildConfigRequest() *ctx.ConfigRequest
	Integrations() []config.ConfigEntry
	GetConfig() databind.YAMLConfig
}

func GetConfigProtocolBuilder(content []byte) Builder {
	val := &discriminator{}
	if err := json.Unmarshal(content, val); err != nil || val.Version == "" {
		return nil
	}
	builderFn := cfgProtocolVersions[val.Version]
	if builderFn == nil {
		builderFn = defaultBuilderFn
	}
	return &builder{
		content: content,
		fn:      builderFn,
	}
}

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
