// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"github.com/newrelic/infrastructure-agent/internal/plugins/darwin"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/process"
	metricsSender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/proxy"
)

func RegisterPlugins(a *agent.Agent) error {
	a.RegisterPlugin(darwin.NewHostinfoPlugin(a.Context,
		common.NewHostInfoCommon(a.Context.Version(), !a.Context.Config().DisableCloudMetadata, a.GetCloudHarvester())))
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
	procSampler := process.NewProcessSampler(a.Context)
	storageSampler := storage.NewSampler(a.Context)
	// nfsSampler := nfs.NewSampler(a.Context)
	networkSampler := network.NewNetworkSampler(a.Context)

	var ntpMonitor metrics.NtpMonitor
	if config.NtpMetrics.Enabled {
		ntpMonitor = metrics.NewNtp(config.NtpMetrics.Pool, config.NtpMetrics.Timeout, config.NtpMetrics.Interval)
	}
	systemSampler := metrics.NewSystemSampler(a.Context, storageSampler, ntpMonitor)

	sender.RegisterSampler(systemSampler)
	sender.RegisterSampler(storageSampler)
	// sender.RegisterSampler(nfsSampler)
	sender.RegisterSampler(networkSampler)
	sender.RegisterSampler(procSampler)

	a.RegisterMetricsSender(sender)

	return nil
}
