// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	metricsSender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/proxy"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	pluginsWindows "github.com/newrelic/infrastructure-agent/internal/plugins/windows"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
)

func RegisterPlugins(a *agent.Agent) error {
	config := a.GetContext().Config()

	if config.IsForwardOnly {
		return nil
	}

	// Enabling the hostinfo plugin will make the host appear in the UI
	a.RegisterPlugin(pluginsWindows.NewHostinfoPlugin(ids.PluginID{"metadata", "system"}, a.Context,
		common.NewHostInfoCommon(a.Context.Version(), !a.Context.Config().DisableCloudMetadata, a.GetCloudHarvester())))
	a.RegisterPlugin(NewHostAliasesPlugin(a.Context, a.GetCloudHarvester()))
	a.RegisterPlugin(NewAgentConfigPlugin(ids.PluginID{"metadata", "agent_config"}, a.Context))
	if config.ProxyConfigPlugin {
		a.RegisterPlugin(proxy.ConfigPlugin(a.Context))
	}

	a.RegisterPlugin(NewCustomAttrsPlugin(a.Context))

	if config.IsSecureForwardOnly {
		// We need heartbeat samples.
		sender := metricsSender.NewSender(a.Context)
		heartBeatSampler := metrics.NewHeartbeatSampler(a.Context)
		sender.RegisterSampler(heartBeatSampler)
		a.RegisterMetricsSender(sender)
		return nil
	}

	a.RegisterPlugin(NewNetworkInterfacePlugin(ids.PluginID{"system", "network_interfaces"}, a.Context))
	a.RegisterPlugin(pluginsWindows.NewServicesPlugin(ids.PluginID{"services", "windows_services"}, a.Context))
	if config.EnableWinUpdatePlugin {
		a.RegisterPlugin(pluginsWindows.NewUpdatesPlugin(ids.PluginID{"packages", "windows_updates"}, a.Context))
	}

	if config.FilesConfigOn {
		a.RegisterPlugin(NewConfigFilePlugin(ids.PluginID{"files", "config"}, a.Context))
	}

	sender := metricsSender.NewSender(a.Context)
	procSampler := metrics.NewProcsMonitor(a.Context)
	storageSampler := storage.NewSampler(a.Context)
	// Prime Storage Sampler, ignoring results
	slog.Debug("Prewarming Sampler Cache.")
	if _, err := storageSampler.Sample(); err != nil {
		slog.WithError(err).Debug("Warming up Storage Sampler Cache.")
	}

	networkSampler := network.NewNetworkSampler(a.Context)
	// Prime Network Sampler, ignoring results
	slog.Debug("Prewarming NetworkSampler Cache.")
	if _, err := networkSampler.Sample(); err != nil {
		slog.WithError(err).Debug("Warming up Network Sampler Cache.")
	}

	var ntpMonitor metrics.NtpMonitor
	if config.NtpMetrics.Enabled {
		ntpMonitor = metrics.NewNtp(config.NtpMetrics.Pool, config.NtpMetrics.Timeout, config.NtpMetrics.Interval)
	}
	systemSampler := metrics.NewSystemSampler(a.Context, storageSampler, ntpMonitor)
	sender.RegisterSampler(systemSampler)
	sender.RegisterSampler(storageSampler)
	sender.RegisterSampler(networkSampler)
	sender.RegisterSampler(procSampler)
	a.RegisterMetricsSender(sender)

	return nil
}
