// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	metricsSender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/proxy"
)

func RegisterPlugins(a *agent.Agent) error {
	a.RegisterPlugin(NewHostAliasesPlugin(a.Context, a.GetCloudHarvester()))
	config := a.Context.Config()

	if config.ProxyConfigPlugin {
		a.RegisterPlugin(proxy.ConfigPlugin(a.Context))
	}
	a.RegisterPlugin(NewCustomAttrsPlugin(a.Context))
	a.RegisterPlugin(NewAgentConfigPlugin(*ids.NewPluginID("metadata", "agent_config"), a.Context))

	if config.FilesConfigOn {
		a.RegisterPlugin(NewConfigFilePlugin(*ids.NewPluginID("files", "config"), a.Context))
	}

	sender := metricsSender.NewSender(a.Context)
	storageSampler := storage.NewSampler(a.Context)
	systemSampler := metrics.NewSystemSampler(a.Context, storageSampler)

	// Prime Storage Sampler, ignoring results
	if !storageSampler.Disabled() {
		slog.Debug("Prewarming Sampler Cache.")
		if _, err := storageSampler.Sample(); err != nil {
			slog.WithError(err).Debug("Warming up Storage Sampler Cache.")
		}
	}

	sender.RegisterSampler(systemSampler)
	sender.RegisterSampler(storageSampler)
	a.RegisterMetricsSender(sender)

	return nil
}
