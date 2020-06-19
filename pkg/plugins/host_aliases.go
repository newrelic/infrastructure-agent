// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

type HostAliasesPlugin struct {
	agent.PluginCommon
	resolver       hostname.Resolver
	cloudAliases   map[string]string
	cloudHarvester cloud.Harvester // Used to get metadata for the instance.
	logger         log.Entry
}

func NewHostAliasesPlugin(ctx agent.AgentContext, cloudHarvester cloud.Harvester) agent.Plugin {
	id := ids.PluginID{
		Category: "metadata",
		Term:     "host_aliases",
	}
	return &HostAliasesPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      id,
			Context: ctx},
		cloudHarvester: cloudHarvester,
		resolver:       ctx.HostnameResolver(),
		logger:         slog.WithField("id", id),
	}
}

func (self *HostAliasesPlugin) getHostAliasesDataset() (dataset agent.PluginInventoryDataset, err error) {
	fullHostname, shortHostname, err := self.resolver.Query()
	if err != nil {
		return nil, fmt.Errorf("error resolving hostname: %s", err)
	}
	if len(fullHostname) == 0 {
		return nil, fmt.Errorf("retrieved empty hostname")
	}

	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  fullHostname,
		Source: sysinfo.HOST_SOURCE_HOSTNAME,
	})

	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  shortHostname,
		Source: sysinfo.HOST_SOURCE_HOSTNAME_SHORT,
	})

	// Retrieve the host alias from config
	if self.Context.Config().DisplayName != "" {
		dataset = append(dataset, sysinfo.HostAliases{
			Alias:  self.Context.Config().DisplayName,
			Source: sysinfo.HOST_SOURCE_DISPLAY_NAME,
		})
	}

	// Retrieve the instance ID if the host happens to be running in a cloud VM. If we hit an
	// error or successfully get the instance ID, stop retrying because it will never change.
	if self.shouldCollectCloudMetadata() {
		err := self.collectCloudMetadata()
		if err != nil {
			self.logger.WithError(err).Debug("Could not retrieve instance ID. Either this is not the cloud or the metadata API returned an error.")
		}
	}

	for key, value := range self.cloudAliases {
		dataset = append(dataset, sysinfo.HostAliases{
			Source: key,
			Alias:  value,
		})
	}

	return
}

// shouldCollectCloudMetadata will check if we should query for the cloud metadata.
func (self *HostAliasesPlugin) shouldCollectCloudMetadata() bool {
	return !self.Context.Config().DisableCloudMetadata &&
		!self.Context.Config().DisableCloudInstanceId &&
		self.cloudHarvester.GetCloudType().ShouldCollect()
}

// Collect cloud metadata and set self.cloudAliases to include whatever we found
func (self *HostAliasesPlugin) collectCloudMetadata() error {
	instanceID, err := self.cloudHarvester.GetInstanceID()
	if err != nil {
		return err
	}

	self.cloudAliases = map[string]string{
		self.cloudHarvester.GetCloudSource(): instanceID,
	}
	return nil
}

func (self *HostAliasesPlugin) Run() {
	refreshTimer := time.NewTicker(1)

	for {
		select {
		case <-refreshTimer.C:
			refreshTimer.Stop()
			refreshTimer = time.NewTicker(config.FREQ_PLUGIN_HOST_ALIASES * time.Second)
			{
				var dataset agent.PluginInventoryDataset
				var err error
				self.logger.Debug("Starting harvest.")
				if dataset, err = self.getHostAliasesDataset(); err != nil {
					self.logger.WithError(err).Error("fetching aliases")
					continue
				}
				self.logger.WithField("dataset", dataset).Debug("Completed harvest, emitting.")
				self.EmitInventory(dataset, self.Context.AgentIdentifier())
				self.logger.Debug("Completed emitting.")
			}
		}
	}
}
