// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package linux

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var sllog = log.WithPlugin("Selinux")
var ErrSELinuxDisabled = errors.New("SELinux status: disabled")

type SELinuxPlugin struct {
	agent.PluginCommon
	frequency      time.Duration
	enableSemodule bool
}

type SELinuxConfigValue struct {
	Key   string `json:"id"`
	Value string `json:"value"`
}

func (configValue SELinuxConfigValue) SortKey() string { return configValue.Key }

type SELinuxPolicyModule struct {
	Key     string `json:"id"`
	Version string `json:"version"`
}

func (policyModule SELinuxPolicyModule) SortKey() string { return policyModule.Key }

// Output we care about - if the label from sestatus output matches a key, we'll
// return its value using this map's value as the ID for the inventory entity.
var SELinuxConfigProperties = map[string]string{
	"SELinux status":          "Status",
	"SELinuxfs mount":         "FSMount",
	"Current mode":            "CurrentMode",
	"Policy version":          "PolicyVersion",
	"Policy from config file": "PolicyLevel",
}

func NewSELinuxPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &SELinuxPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.SelinuxIntervalSec,
			config.FREQ_MINIMUM_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_SELINUX_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
		enableSemodule: ctx.Config().SelinuxEnableSemodule,
	}
}

// getDataset collects the various information we want to report about SELinux and returns a separate dataset for each type of output:
//
//	    basicData: Overall SELinux status - whether it's running, what mode it's in, etc.
//	   policyData: Individual SELinux policy flags - a high-level overview of SELinux configuration
//	policyModules: Listing of policy modules in use and which version of modules are active
func (seLinuxPlugin *SELinuxPlugin) getDataset() (types.PluginInventoryDataset, types.PluginInventoryDataset, types.PluginInventoryDataset, error) {
	// Get basic selinux status data using sestatus. If selinux isn't enabled or installed, this will fail.
	var basicData = types.PluginInventoryDataset{}
	var policyData = types.PluginInventoryDataset{}
	var policyModules = types.PluginInventoryDataset{}
	var err error
	output, err := helpers.RunCommand("sestatus", "", "-b")
	if err != nil {
		return nil, nil, nil, err
	}

	if basicData, policyData, err = seLinuxPlugin.parseSestatusOutput(output); err != nil {
		return nil, nil, nil, err
	}

	if seLinuxPlugin.enableSemodule {
		// Get versions of policy modules installed using semodule
		if output, err = helpers.RunCommand("semodule", "", "-l"); err != nil {
			return nil, nil, nil, err
		}

		if policyModules, err = seLinuxPlugin.parseSemoduleOutput(output); err != nil {
			return nil, nil, nil, err
		}
	}
	return basicData, policyData, policyModules, err
}

func (seLinuxPlugin *SELinuxPlugin) parseSestatusOutput(output string) (basicResult types.PluginInventoryDataset, policyResult types.PluginInventoryDataset, err error) {
	labelRegex, err := regexp.Compile(`([^:]*):\s+(.*)`)
	if err != nil {
		return
	}

	policyBooleanRegex, err := regexp.Compile(`(\S+)\s+(\S+)`)
	if err != nil {
		return
	}

	scanningPolicyBooleans := false
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if line == "Policy booleans:" {
			// We've reached the chunk of output which lists all the various policy switches, and they have a different format.
			scanningPolicyBooleans = true
		} else if scanningPolicyBooleans {
			// We're going through policy booleans of the format "policy_name       on/off"
			policyBooleanMatches := policyBooleanRegex.FindAllStringSubmatch(line, -1)
			// if policyBooleanMatches != nil {
			// 	label := policyBooleanMatches[0][1]
			// 	value := policyBooleanMatches[0][2]
			// 	policyResult = append(policyResult, SELinuxConfigValue{fmt.Sprintf("%s", label), value})
			// }
			policyResult = seLinuxPlugin.parsePolicyBooleans(policyBooleanMatches)

		} else {
			// We're scanning general status output of the format "Label:        value"
			labelMatches := labelRegex.FindAllStringSubmatch(line, -1)
			basicResult, err = seLinuxPlugin.parseLabel(labelMatches)
			if err != nil {
				return nil, nil, err
			}
			// if labelMatches != nil {
			// 	label := labelMatches[0][1]
			// 	value := labelMatches[0][2]
			// 	if label == "SELinux status" && value == "disabled" {
			// 		return nil, nil, ErrSELinuxDisabled
			// 	}
			// 	if entityID, ok := SELinuxConfigProperties[label]; ok {
			// 		basicResult = append(basicResult, SELinuxConfigValue{entityID, value})
			// 	}
			// }
		}
	}

	return
}

func (seLinuxPlugin *SELinuxPlugin) parsePolicyBooleans(policyBooleanMatches [][]string) (policyResult types.PluginInventoryDataset) {
	if policyBooleanMatches != nil {
		label := policyBooleanMatches[0][1]
		value := policyBooleanMatches[0][2]
		policyResult = append(policyResult, SELinuxConfigValue{fmt.Sprintf("%s", label), value})
	}
	return
}

func (seLinuxPlugin *SELinuxPlugin) parseLabel(labelMatches [][]string) (basicResult types.PluginInventoryDataset, err error) {
	if labelMatches != nil {
		label := labelMatches[0][1]
		value := labelMatches[0][2]
		if label == "SELinux status" && value == "disabled" {
			return nil, ErrSELinuxDisabled
		}
		if entityID, ok := SELinuxConfigProperties[label]; ok {
			basicResult = append(basicResult, SELinuxConfigValue{entityID, value})
		}
	}
	return
}

func (seLinuxPlugin *SELinuxPlugin) sELinuxActive() bool {
	output, err := helpers.RunCommand("sestatus", "", "-b")
	if err != nil {
		sllog.WithError(err).Debug("Unable to use SELinux.")
		return false
	}

	if _, _, err = seLinuxPlugin.parseSestatusOutput(output); err == ErrSELinuxDisabled {
		sllog.WithError(err).Debug("Unable to use SELinux.")
	}
	return err == nil
}

func (seLinuxPlugin *SELinuxPlugin) parseSemoduleOutput(output string) (types.PluginInventoryDataset, error) {
	var result = types.PluginInventoryDataset{}
	var err error
	// Capture "zero-or-more elements" of whitespaces and non-whitespaces for the optional version field
	moduleRegex := regexp.MustCompile(`(\S+)\s*(\S*)`)
	scanner := bufio.NewScanner(strings.NewReader(output))
	const captureGroupsMinLen = 2
	for scanner.Scan() {
		line := scanner.Text()

		moduleMatches := moduleRegex.FindAllStringSubmatch(line, -1)
		if moduleMatches != nil {
			id := moduleMatches[0][1]

			// Guard against the second capture group not existing
			version := ""
			if len(moduleMatches[0]) > captureGroupsMinLen {
				version = moduleMatches[0][2]
			}

			result = append(result, SELinuxPolicyModule{id, version})
		}
	}
	return result, err
}

func (seLinuxPlugin *SELinuxPlugin) Run() {
	if seLinuxPlugin.frequency <= config.FREQ_DISABLE_SAMPLING {
		sllog.Debug("Disabled.")
		return
	}

	if seLinuxPlugin.sELinuxActive() {
		seLinuxPlugin.RunCommands()
	} else {
		seLinuxPlugin.Unregister()
	}
}

func (seLinuxPlugin *SELinuxPlugin) RunCommands() {
	if seLinuxPlugin.enableSemodule {
		distro := helpers.GetLinuxDistro()
		if distro == helpers.LINUX_REDHAT || distro == helpers.LINUX_AWS_REDHAT {
			sllog.Warn("enabling 'semodule' may report performance issues in RedHat-based distributions")
		}
	}

	refreshTimer := time.NewTicker(seLinuxPlugin.frequency)
	for {
		basicData, policyData, policyModules, err := seLinuxPlugin.getDataset()
		if err != nil {
			sllog.WithError(err).Error("selinux can't get dataset")
		}

		entity := entity.NewFromNameWithoutID(seLinuxPlugin.Context.EntityKey())
		seLinuxPlugin.Context.SendData(types.NewPluginOutput(seLinuxPlugin.Id(), entity, basicData))
		seLinuxPlugin.Context.SendData(types.NewPluginOutput(ids.PluginID{Category: seLinuxPlugin.ID.Category, Term: fmt.Sprintf("%s-policies", seLinuxPlugin.ID.Term)}, entity, policyData))

		if seLinuxPlugin.enableSemodule {
			seLinuxPlugin.Context.SendData(types.NewPluginOutput(ids.PluginID{Category: seLinuxPlugin.ID.Category, Term: fmt.Sprintf("%s-modules", seLinuxPlugin.ID.Term)}, entity, policyModules))
		}

		<-refreshTimer.C
	}
}
