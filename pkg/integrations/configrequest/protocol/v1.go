package protocol

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
)

/* The protocol is used to register integrations without the need to use config yaml files.
It wraps the objects used in the configuration files to define variables and integrations.
Config protocol example.
{
	"config_protocol_version": "1",
	"action": "register_config",
	"config_name": "myconfig",
	"config": {
	  "variables": {
		"creds": {
		  "vault": {
			"http": {
			  "url": "http://my.vault.host/v1/newengine/data/secret",
			  "headers": {
				"X-Vault-Token": "my-vault-token"
			  }
			}
		  }
		}
	  },
	  "integrations": [
		{
		  "name": "nri-mysql",
		  "interval": "15s",
		  "env": {
			"PORT": "3306",
			"USERNAME": "${creds.username}",
			"PASSWORD": "${creds.password}"
		  }
		},
		{
		  "name": "long-running-integration",
		  "timeout": "0"
		  "exec": "python /opt/integrations/my-script.py --host=127.0.0.1"
		}
	  ]
	}
}
*/

type v1 struct {
	Action     string `json:"action"`
	ConfigName string `json:"config_name"`
	Config     struct {
		Databind     databind.YAMLAgentConfig `json:",inline"`
		Integrations []config.ConfigEntry     `json:"integrations"`
	} `json:"config"`
}

func (cfgProtocol *v1) withConfig(databind databind.YAMLAgentConfig, integrations []config.ConfigEntry) *v1 {
	cfgProtocol.Config.Integrations = integrations
	cfgProtocol.Config.Databind = databind
	return cfgProtocol
}

func (cfgProtocol *v1) Version() int {
	return 1
}

func (cfgProtocol *v1) Name() string {
	return cfgProtocol.ConfigName
}

func (cfgProtocol *v1) GetConfig() databind.YAMLConfig {
	return databind.YAMLConfig{YAMLAgentConfig: cfgProtocol.Config.Databind}
}

func (cfgProtocol *v1) Integrations() []config.ConfigEntry {
	return cfgProtocol.Config.Integrations
}

func (cfgProtocol *v1) validate() error {
	if cfgProtocol.ConfigName == "" {
		return fmt.Errorf("config_name cannot be empty")
	}
	if cfgProtocol.Action == "" {
		return fmt.Errorf("action cannot be empty")
	}
	if len(cfgProtocol.Config.Integrations) == 0 {
		return fmt.Errorf("configs entries definitions cannot be empty")
	}
	return nil
}
