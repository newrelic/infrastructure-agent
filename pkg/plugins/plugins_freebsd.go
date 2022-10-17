// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/freebsd"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/process"
	metricsSender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
)

func RegisterPlugins(a *agent.Agent) error {

	config := a.GetContext().Config()

	a.RegisterPlugin(freebsd.NewHostinfoPlugin(a.Context, a.GetCloudHarvester()))
	a.RegisterPlugin(NewHostAliasesPlugin(a.Context, a.GetCloudHarvester()))

	storageSampler := storage.NewSampler(a.Context)
	sender := metricsSender.NewSender(a.Context)
	var ntpMonitor metrics.NtpMonitor
	if config.Ntp.Enabled {
		ntpMonitor = metrics.NewNtp(config.Ntp.Pool, config.Ntp.Timeout, config.Ntp.Interval)
	}
	systemSampler := metrics.NewSystemSampler(a.Context, storageSampler, ntpMonitor)

	networkSampler := network.NewNetworkSampler(a.Context)
	procSampler := process.NewProcessSampler(a.Context)

	sender.RegisterSampler(storageSampler)
	sender.RegisterSampler(systemSampler)
	sender.RegisterSampler(networkSampler)
	sender.RegisterSampler(procSampler)

	a.RegisterMetricsSender(sender)

	return nil
}
