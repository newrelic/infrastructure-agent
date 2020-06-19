// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package plugins

import (
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

type pluginRetrieve func() []agent.Plugin

// Plugin that links integrations to the pod they are monitoring
type K8sIntegrationsPlugin struct {
	agent.PluginCommon
	pluginsRetrieve pluginRetrieve
	frequency       time.Duration
}

func NewK8sIntegrationsPlugin(ctx agent.AgentContext, pluginRetrieveFn pluginRetrieve) agent.Plugin {
	cfg := ctx.Config()
	p := &K8sIntegrationsPlugin{
		PluginCommon: agent.PluginCommon{
			ID: ids.PluginID{
				Category: "metadata",
				Term:     "k8s_integrations",
			},
			Context: ctx,
		},
		pluginsRetrieve: pluginRetrieveFn,
		frequency: config.ValidateConfigFrequencySetting(
			cfg.K8sIntegrationSamplesIntervalSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_K8S_INTEGRATION_SAMPLES_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}

	return p
}

func (kip *K8sIntegrationsPlugin) Run() {
	kip.Context.AddReconnecting(kip)
	entityKey := kip.Context.AgentIdentifier()

	refreshTimer := time.NewTicker(kip.frequency)
	for range refreshTimer.C {
		for _, plugin := range kip.pluginsRetrieve() {
			if plugin.IsExternal() {
				kip.EmitEvent(kip.k8sIntegrationSample(plugin.GetExternalPluginName()), entity.Key(entityKey))
			}
		}
	}
}

func (kip *K8sIntegrationsPlugin) k8sIntegrationSample(integrationName string) map[string]interface{} {
	return map[string]interface{}{
		"integrationName": integrationName,
		"eventType":       "K8sIntegrationSample",
	}
}
