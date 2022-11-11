// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	agnt "github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"
	config2 "github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/process"
	metricsSender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage/nfs"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/proxy"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
)

func registerForwarderHeartbeat(a *agnt.Agent) {
	sender := metricsSender.NewSender(a.Context)
	heartBeatSampler := metrics.NewHeartbeatSampler(a.Context)
	sender.RegisterSampler(heartBeatSampler)
	a.RegisterMetricsSender(sender)
}

func RegisterPlugins(agent *agnt.Agent) error {
	config := agent.GetContext().Config()
	// Deprecating a pluging causes the agent to delete its inventory
	agent.DeprecatePlugin(ids.PluginID{"metadata", "cloud_instance"})
	agent.DeprecatePlugin(ids.PluginID{"metadata", "cloud_ami"})
	agent.DeprecatePlugin(ids.PluginID{"hostinfo", "host_info"})
	agent.DeprecatePlugin(ids.PluginID{"hostinfo", "hostinfo"})
	agent.DeprecatePlugin(ids.PluginID{"services", "sysv_init"})
	agent.DeprecatePlugin(ids.PluginID{"services", "docker"})

	if config.K8sIntegration {
		agent.RegisterPlugin(NewK8sIntegrationsPlugin(agent.Context, agent.Plugins))
	}

	if config.IsForwardOnly {
		return nil
	}

	agent.RegisterPlugin(NewCustomAttrsPlugin(agent.Context))

	// Enabling the hostinfo plugin will make the host appear in the UI
	agent.RegisterPlugin(pluginsLinux.NewHostinfoPlugin(agent.Context,
		common.NewHostInfoCommon(agent.Context.Version(), !agent.Context.Config().DisableCloudMetadata, agent.GetCloudHarvester())))

	agent.RegisterPlugin(NewHostAliasesPlugin(agent.Context, agent.GetCloudHarvester()))
	agent.RegisterPlugin(NewAgentConfigPlugin(ids.PluginID{"metadata", "agent_config"}, agent.Context))
	if config.ProxyConfigPlugin {
		agent.RegisterPlugin(proxy.ConfigPlugin(agent.Context))
	}

	if config.IsSecureForwardOnly {
		registerForwarderHeartbeat(agent)
		return nil
	}

	// register remaining plugins
	if !config.IsContainerized {
		// register our plugins
		agent.RegisterPlugin(pluginsLinux.NewUpstartPlugin(ids.PluginID{"services", "upstart"}, agent.Context))
		agent.RegisterPlugin(pluginsLinux.NewSystemdPlugin(agent.Context))
		agent.RegisterPlugin(pluginsLinux.NewFacterPlugin(agent.Context))
		if config.FilesConfigOn {
			agent.RegisterPlugin(NewConfigFilePlugin(ids.PluginID{"files", "config"}, agent.Context))
		}
		agent.RegisterPlugin(pluginsLinux.NewUsersPlugin(agent.Context))
		agent.RegisterPlugin(pluginsLinux.NewDaemontoolsPlugin(ids.PluginID{"services", "daemontools"}, agent.Context))
		agent.RegisterPlugin(pluginsLinux.NewSupervisorPlugin(ids.PluginID{"services", "supervisord"}, agent.Context))
		agent.RegisterPlugin(NewNetworkInterfacePlugin(ids.PluginID{"system", "network_interfaces"}, agent.Context))

		if config.RunMode == config2.ModeRoot || config.RunMode == config2.ModePrivileged {
			id := ids.PluginID{"kernel", "sysctl"}
			if config.SysctlFSNotify {
				p, err := pluginsLinux.NewSysctlSubscriberMonitor(id, agent.Context)
				if err != nil {
					slog.WithField("plugin", id.String()).Error("cannot initialize plugin")
				} else {
					agent.RegisterPlugin(p)
				}
			} else {
				agent.RegisterPlugin(pluginsLinux.NewSysctlPollingMonitor(id, agent.Context))
			}
			agent.RegisterPlugin(pluginsLinux.NewKernelModulesPlugin(ids.PluginID{"kernel", "modules"}, agent.Context))
			agent.RegisterPlugin(pluginsLinux.NewSysvInitPlugin(ids.PluginID{"services", "pidfile"}, agent.Context))
			agent.RegisterPlugin(pluginsLinux.NewSshdConfigPlugin(ids.PluginID{"config", "sshd"}, agent.Context))

			// platform specific plugins
			switch helpers.GetLinuxDistro() {
			case helpers.LINUX_DEBIAN:
				slog.Debug("Registering Ubuntu/Debian plugins.")
				agent.RegisterPlugin(pluginsLinux.NewDpkgPlugin(ids.PluginID{"packages", "dpkg"}, agent.Context))

			case helpers.LINUX_REDHAT, helpers.LINUX_AWS_REDHAT, helpers.LINUX_SUSE:
				slog.Debug("Registering RPM plugins.")
				agent.RegisterPlugin(pluginsLinux.NewRpmPlugin(agent.Context))
			}
		}

		if config.RunMode == config2.ModeRoot {
			agent.RegisterPlugin(pluginsLinux.NewSELinuxPlugin(ids.PluginID{"config", "selinux"}, agent.Context))
		}

		if agent.GetCloudHarvester().GetCloudType() == cloud.TypeAWS {
			agent.RegisterPlugin(pluginsLinux.NewCloudSecurityGroupsPlugin(ids.PluginID{"metadata", "cloud_security_groups"}, agent.Context, agent.GetCloudHarvester()))
		}
	}

	sender := metricsSender.NewSender(agent.Context)
	procSampler := process.NewProcessSampler(agent.Context)
	storageSampler := storage.NewSampler(agent.Context)
	nfsSampler := nfs.NewSampler(agent.Context)
	networkSampler := network.NewNetworkSampler(agent.Context)

	var ntpMonitor metrics.NtpMonitor
	if config.NtpMetrics.Enabled {
		ntpMonitor = metrics.NewNtp(config.NtpMetrics.Pool, config.NtpMetrics.Timeout, config.NtpMetrics.Interval)
	}
	systemSampler := metrics.NewSystemSampler(agent.Context, storageSampler, ntpMonitor)

	// Prime Storage Sampler, ignoring results
	if !storageSampler.Disabled() {
		slog.Debug("Prewarming Sampler Cache.")
		if _, err := storageSampler.Sample(); err != nil {
			slog.WithError(err).Debug("Warming up Storage Sampler Cache.")
		}
	}

	// Prime Network Sampler, ignoring results
	if !networkSampler.Disabled() {
		slog.Debug("Prewarming NetworkSampler Cache.")
		if _, err := networkSampler.Sample(); err != nil {
			slog.WithError(err).Debug("Warming up Network Sampler Cache.")
		}
	}

	sender.RegisterSampler(systemSampler)
	sender.RegisterSampler(storageSampler)
	sender.RegisterSampler(nfsSampler)
	sender.RegisterSampler(networkSampler)
	sender.RegisterSampler(procSampler)

	agent.RegisterMetricsSender(sender)

	return nil
}
