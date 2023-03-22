// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package linux

import (
	"errors"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var csglog = log.WithPlugin("CloudSecurityGroup")

type CloudSecurityGroupsPlugin struct {
	agent.PluginCommon
	harvester        cloud.Harvester
	frequency        time.Duration
	disableKeepAlive bool
}

type CloudSecurityGroup struct {
	SecurityGroup string `json:"id"`
}

func (c CloudSecurityGroup) SortKey() string {
	return c.SecurityGroup
}

func NewCloudSecurityGroupsPlugin(id ids.PluginID, ctx agent.AgentContext, harvester cloud.Harvester) agent.Plugin {
	cfg := ctx.Config()
	return &CloudSecurityGroupsPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.CloudSecurityGroupRefreshSec,
			config.FREQ_MINIMUM_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_CLOUD_SECURITY_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
		disableKeepAlive: cfg.CloudMetadataDisableKeepAlive,
		harvester:        harvester,
	}
}

func (p *CloudSecurityGroupsPlugin) getCloudSecurityGroupsDataset() (dataset types.PluginInventoryDataset, err error) {
	var h cloud.Harvester
	h, err = p.harvester.GetHarvester()
	if err != nil {
		return dataset, err
	}

	awsH, ok := h.(*cloud.AWSHarvester)
	if !ok {
		err = errors.New("not an AWS Harvester")
		return dataset, err
	}

	var cloudSecurityGroups string
	if cloudSecurityGroups, err = awsH.GetAWSMetadataValue("security-groups", p.disableKeepAlive); err != nil {
		return dataset, err
	}

	for _, cloudSecurityGroup := range strings.Split(cloudSecurityGroups, "\n") {
		dataset = append(dataset, CloudSecurityGroup{cloudSecurityGroup})
	}

	return dataset, err
}

func (p *CloudSecurityGroupsPlugin) Run() {
	if p.Context.Config().DisableCloudMetadata {
		csglog.Debug("Cloud security group disabled by disable_cloud_metadata.")
		return
	}

	if p.frequency <= config.FREQ_DISABLE_SAMPLING {
		csglog.Debug("Disabled.")
		return
	}

	refreshTimer := time.NewTicker(1)

	for {
		select {
		case <-refreshTimer.C:
			refreshTimer.Stop()
			refreshTimer = time.NewTicker(p.frequency)
			{
				var dataset types.PluginInventoryDataset
				var err error
				if dataset, err = p.getCloudSecurityGroupsDataset(); err != nil {
					// Silence errors here, they are only advisory and the function returns an
					// error when not on a cloud. We're just going to turn this off for now.
					refreshTimer.Stop()
					csglog.WithError(err).Debug("Reading security groups.")
					return
				}
				p.EmitInventory(dataset, entity.NewFromNameWithoutID(p.Context.EntityKey()))
			}
		}
	}
}
