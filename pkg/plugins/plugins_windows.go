// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	metricsSender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/proxy"

	agnt "github.com/newrelic/infrastructure-agent/internal/agent"
	pluginsWindows "github.com/newrelic/infrastructure-agent/internal/plugins/windows"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
)

func RegisterPlugins(agent *agnt.Agent) error {
	config := agent.GetContext().Config()

	if config.IsForwardOnly {
		return nil
	}

	// Enabling the hostinfo plugin will make the host appear in the UI
	agent.RegisterPlugin(pluginsWindows.NewHostinfoPlugin(ids.PluginID{"metadata", "system"}, agent.Context, agent.GetCloudHarvester()))
	agent.RegisterPlugin(NewHostAliasesPlugin(agent.Context, agent.GetCloudHarvester()))
	agent.RegisterPlugin(NewAgentConfigPlugin(ids.PluginID{"metadata", "agent_config"}, agent.Context))
	if config.ProxyConfigPlugin {
		agent.RegisterPlugin(proxy.ConfigPlugin(agent.Context))
	}

	agent.RegisterPlugin(NewCustomAttrsPlugin(agent.Context))

	if config.HTTPServerEnabled {
		agent.RegisterPlugin(NewHTTPServerPlugin(agent.Context, config.HTTPServerHost, config.HTTPServerPort))
	}

	if config.IsSecureForwardOnly {
		// We need heartbeat samples.
		sender := metricsSender.NewSender(agent.Context)
		heartBeatSampler := metrics.NewHeartbeatSampler(agent.Context)
		sender.RegisterSampler(heartBeatSampler)
		agent.RegisterMetricsSender(sender)
		return nil
	}

	agent.RegisterPlugin(NewNetworkInterfacePlugin(ids.PluginID{"system", "network_interfaces"}, agent.Context))
	agent.RegisterPlugin(pluginsWindows.NewServicesPlugin(ids.PluginID{"services", "windows_services"}, agent.Context))
	if config.EnableWinUpdatePlugin {
		agent.RegisterPlugin(pluginsWindows.NewUpdatesPlugin(ids.PluginID{"packages", "windows_updates"}, agent.Context))
	}

	if config.FilesConfigOn {
		agent.RegisterPlugin(NewConfigFilePlugin(ids.PluginID{"files", "config"}, agent.Context))
	}

	sender := metricsSender.NewSender(agent.Context)
	procSampler := metrics.NewProcsMonitor(agent.Context)
	storageSampler := storage.NewSampler(agent.Context)
	// Prime Storage Sampler, ignoring results
	slog.Debug("Prewarming Sampler Cache.")
	if _, err := storageSampler.Sample(); err != nil {
		slog.WithError(err).Debug("Warming up Storage Sampler Cache.")
	}

	networkSampler := network.NewNetworkSampler(agent.Context)
	// Prime Network Sampler, ignoring results
	slog.Debug("Prewarming NetworkSampler Cache.")
	if _, err := networkSampler.Sample(); err != nil {
		slog.WithError(err).Debug("Warming up Network Sampler Cache.")
	}

	systemSampler := metrics.NewSystemSampler(agent.Context, storageSampler)
	sender.RegisterSampler(systemSampler)
	sender.RegisterSampler(storageSampler)
	sender.RegisterSampler(networkSampler)
	sender.RegisterSampler(procSampler)
	agent.RegisterMetricsSender(sender)

	return nil
}
