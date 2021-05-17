package protocol

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track/ctx"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
)

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

func (cfgProtocol *v1) hash() string {
	return fmt.Sprintf("%v%v", cfgProtocol.Config.Databind, cfgProtocol.Config.Integrations)
}

func (cfgProtocol *v1) BuildConfigRequest() *ctx.ConfigRequest {
	return &ctx.ConfigRequest{
		ConfigName: cfgProtocol.ConfigName,
		ConfigHash: cfgProtocol.hash(),
	}
}

func (cfgProtocol *v1) GetConfig() databind.YAMLConfig {
	return databind.YAMLConfig{YAMLAgentConfig: cfgProtocol.Config.Databind}
}

func (cfgProtocol *v1) Integrations() []config.ConfigEntry {
	return cfgProtocol.Config.Integrations
}
