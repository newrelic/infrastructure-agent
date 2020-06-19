// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package windows

import (
	"fmt"
	"time"

	"github.com/StackExchange/wmi"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

var ulog = log.WithComponent("UpdatesPlugin")

type UpdatesPlugin struct {
	agent.PluginCommon
	frequency time.Duration
}

// See https://msdn.microsoft.com/en-us/library/aa394391(v=vs.85).aspx
type Win32_QuickFixEngineering struct {
	HotFixID    string `json:"id"`
	Description string `json:"description"`
	Caption     string `json:"knowledgebase_url"`
	InstalledOn string `json:"installed_time"`
}

func (self Win32_QuickFixEngineering) SortKey() string {
	return self.HotFixID
}

func NewUpdatesPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &UpdatesPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.WindowsUpdatesRefreshSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_WINDOWS_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self *UpdatesPlugin) getDataset() (result agent.PluginInventoryDataset, err error) {
	var wmiResults []Win32_QuickFixEngineering
	wmiQuery := wmi.CreateQuery(&wmiResults, "")
	if err = wmi.QueryNamespace(wmiQuery, &wmiResults, config.DefaultWMINamespace); err != nil {
		return result, fmt.Errorf("Error querying WMI: %s", err)
	}

	for _, wmiResult := range wmiResults {
		result = append(result, wmiResult)
	}
	return
}

func (self *UpdatesPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		ulog.Debug("Disabled.")
		return
	}

	// Introduce some jitter to wait randomly before reporting based on frequency time
	time.Sleep(config.JitterFrequency(self.frequency))

	refreshTimer := time.NewTicker(self.frequency)
	for {
		dataset, err := self.getDataset()
		if err != nil {
			ulog.WithError(err).Error("updates plugin can't get dataset")
		}
		self.EmitInventory(dataset, self.Context.AgentIdentifier())
		<-refreshTimer.C
	}
}
