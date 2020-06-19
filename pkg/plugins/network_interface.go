// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	network_helpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
)

type NetworkInterfaceData struct {
	InterfaceName   string `json:"interfaceName"`
	HardwareAddress string `json:"hardwareAddress"`
	IpV4Address     string `json:"ipV4Address,omitempty"`
	IpV6Address     string `json:"ipV6Address,omitempty"`
}

func (self NetworkInterfaceData) SortKey() string {
	return self.InterfaceName
}

type NetworkInterfacePlugin struct {
	agent.PluginCommon
	frequency               time.Duration                      // Plugin emit interval
	networkInterfaceFilters map[string][]string                // Controls which interfaces to ignore
	getInterfaces           network_helpers.InterfacesProvider // Provider for []net.InterfaceStat
}

func NewNetworkInterfacePlugin(id ids.PluginID, ctx agent.AgentContext) *NetworkInterfacePlugin {
	var cfg *config.Config
	if ctx != nil {
		cfg = ctx.Config()
	}

	var filters map[string][]string
	if cfg != nil {
		filters = cfg.NetworkInterfaceFilters
	}

	plugin := &NetworkInterfacePlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.NetworkInterfaceIntervalSec,
			config.FREQ_MINIMUM_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_NETWORK_INTERFACE_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
		networkInterfaceFilters: filters,
	}

	return plugin.WithInterfacesProvider(network_helpers.GopsutilInterfacesProvider)
}

func (self *NetworkInterfacePlugin) WithInterfacesProvider(p network_helpers.InterfacesProvider) *NetworkInterfacePlugin {
	self.getInterfaces = p
	return self
}

func (self *NetworkInterfacePlugin) getNetworkInterfaceData() (agent.PluginInventoryDataset, error) {
	var dataset agent.PluginInventoryDataset

	interfaces, err := self.getInterfaces()
	if err != nil {
		return nil, err
	}

	for _, ni := range interfaces {
		if network_helpers.ShouldIgnoreInterface(self.networkInterfaceFilters, ni.Name) {
			continue
		}

		ipv4, ipv6 := network_helpers.IPAddressesByType(ni.Addrs)

		dataset = append(dataset, NetworkInterfaceData{
			InterfaceName:   ni.Name,
			HardwareAddress: ni.HardwareAddr,
			IpV4Address:     ipv4,
			IpV6Address:     ipv6,
		})
	}

	return dataset, nil
}

func (self *NetworkInterfacePlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		slog.WithPlugin(self.Id().String()).Debug("Disabled.")
		return
	}

	ticker := time.NewTicker(1)
	for {
		select {
		case <-ticker.C:
			ticker.Stop()
			ticker = time.NewTicker(self.frequency)

			dataset, err := self.getNetworkInterfaceData()
			if err != nil {
				slog.WithError(err).WithPlugin(self.Id().String()).Error("fetching network interface data")
			}
			self.EmitInventory(dataset, self.Context.AgentIdentifier())
		}
	}
}
