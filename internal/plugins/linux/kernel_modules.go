// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var kmlog = log.WithPlugin("KernelModules")

type KernelModulesPlugin struct {
	agent.PluginCommon
	loadedModules map[string]KernelModule
	needsFlush    bool
	frequency     time.Duration
}

type KernelModule struct {
	Name        string `json:"id"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

func (self KernelModule) SortKey() string {
	return self.Name
}

func NewKernelModulesPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &KernelModulesPlugin{
		PluginCommon:  agent.PluginCommon{ID: id, Context: ctx},
		loadedModules: make(map[string]KernelModule),
		needsFlush:    true,
		frequency: config.ValidateConfigFrequencySetting(
			cfg.KernelModulesRefreshSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_KERNEL_MODULES_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self KernelModulesPlugin) getKernelModulesDataset() agent.PluginInventoryDataset {
	var dataset agent.PluginInventoryDataset

	for _, v := range self.loadedModules {
		dataset = append(dataset, v)
	}

	return dataset
}

func (self *KernelModulesPlugin) populateKernelModuleInfo(modInfo *KernelModule) (err error) {
	output, err := helpers.RunCommand("/sbin/modinfo", "", modInfo.Name)
	if err != nil {
		err = fmt.Errorf("Unable to get module info for '%s': %s", modInfo.Name, err)
		return
	}

	reModuleInfo := regexp.MustCompile(`^(filename|version|description):\s+(.*)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := reModuleInfo.FindStringSubmatch(line)

		if len(matches) == 3 {
			switch matches[1] {
			case "version":
				modInfo.Version = matches[2]
			case "description":
				modInfo.Description = matches[2]
			}
		}
	}

	return
}

func (self *KernelModulesPlugin) getKernelModuleStatus() (err error) {
	output, err := helpers.RunCommand("/sbin/lsmod", "")
	if err != nil {
		err = fmt.Errorf("Unable to list kernel modules. err: %s ouput: %s", err, output)
		return
	}

	foundModuleList := make(map[string]bool)

	reModule := regexp.MustCompile(`^(\S+)\s+\d+`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := reModule.FindStringSubmatch(line)

		if len(matches) == 2 {
			foundModuleList[matches[1]] = true
		}
	}

	return self.processUpdates(foundModuleList)
}

// processUpdates implements the logic for determining detecting kernel module
// install and uninstall by processing the list of modules from lsmod
func (self *KernelModulesPlugin) processUpdates(seenModules map[string]bool) (err error) {
	// Check for removals - iterate over the map of modules we already know about
	// if it doesn't exist in the seenModule list (our more recent lsmod) then
	// remove it from the loadedModules map
	for k := range self.loadedModules {
		if _, ok := seenModules[k]; !ok {
			delete(self.loadedModules, k)
			self.needsFlush = true
		}
	}

	// Check for additions - the inverse of above, if a module we just saw isn't
	// in the global map of loaded modules fetch its info an add it to the map
	for k := range seenModules {
		if _, ok := self.loadedModules[k]; !ok {
			module := KernelModule{Name: k}
			err = self.populateKernelModuleInfo(&module)
			self.loadedModules[k] = module
			if err != nil {
				kmlog.WithError(err).Error("processing update for kernel module")
				err = nil
				continue
			}
			self.needsFlush = true
		}
	}
	return
}

func (self *KernelModulesPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		kmlog.Debug("Disabled.")
		return
	}

	if _, err := os.Stat(helpers.HostProc("/modules")); err == nil {
		first := true
		for {
			if first {
				first = false
			} else {
				time.Sleep(self.frequency)
			}
			err := self.getKernelModuleStatus()
			if err != nil {
				kmlog.WithError(err).Error("getting kernel module status")
			} else {
				if self.needsFlush {
					self.EmitInventory(self.getKernelModulesDataset(), self.Context.AgentIdentifier())
					self.needsFlush = false
				}
			}
		}
	} else {
		kmlog.Warn("Kernel does not appear to allow module loading, kernel modules will not be monitored.")
		self.Unregister()
	}
}
