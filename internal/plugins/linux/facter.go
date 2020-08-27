// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var flog = log.WithPlugin("Facter")
var ignored_facts = []string{"used_bytes", "available_bytes", "load_averages", "used", "capacity", "available", "system_uptime", "uptime_seconds", "uptime", "uptime_days", "uptime_hours", "memoryfree", "memoryfree_mb", "swapfree", "swapfree_mb", "last_run", "puppet_agent_pid"}

type Facter interface {
	Initialize() error
	Facts() (map[string]FacterItem, error)
}

type FacterPlugin struct {
	agent.PluginCommon
	facter    Facter
	frequency time.Duration
}

type FacterItem struct {
	Name  string      `json:"id"`
	Value interface{} `json:"value"`
}

func (fi FacterItem) SortKey() string {
	return fi.Name
}

func NewFacterPlugin(ctx agent.AgentContext) *FacterPlugin {
	id := ids.PluginID{"metadata", "facter_facts"}
	cfg := ctx.Config()
	return &FacterPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		facter: &FacterClient{
			homeDir: cfg.FacterHomeDir,
		},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.FacterIntervalSec,
			config.FREQ_MINIMUM_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_FACTER_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self *FacterPlugin) CanRun() bool {
	err := self.facter.Initialize()
	if err != nil {
		return false
	}
	_, err = self.facter.Facts()
	return err == nil
}

func (self *FacterPlugin) Data() (agent.PluginInventoryDataset, error) {
	services, err := self.facter.Facts()
	if err != nil {
		return nil, err
	}
	a := agent.PluginInventoryDataset{}
	for _, svc := range services {
		a = append(a, svc)
	}
	return a, nil
}

func (self *FacterPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		flog.Debug("Disabled.")
		return
	}

	if !self.CanRun() {
		self.Unregister()
		return
	}
	for {
		data, err := self.Data()
		if err != nil {
			time.Sleep(self.frequency)
			continue
		}
		self.EmitInventory(data, self.Context.AgentIdentifier())
		time.Sleep(self.frequency)
	}
}

type FacterClient struct {
	homeDir string
}

func (self *FacterClient) Initialize() error {
	_, err := exec.LookPath("facter")
	return err
}

func (self *FacterClient) Facts() (map[string]FacterItem, error) {
	output, err := self.runFacter()
	if err != nil {
		return nil, fmt.Errorf("Unable to run facter: %s", err)
	}
	return parseFacts(output)
}

func parseFacts(output []byte) (facts map[string]FacterItem, err error) {
	output = helpers.ISO8601RE.ReplaceAll(output, []byte("(timestamp suppressed)"))
	var facterOutput map[string]interface{}
	err = json.Unmarshal(output, &facterOutput)
	if err != nil {
		fmt.Println("error unmarshalling output:")
		return nil, err
	}

	json_map := make(map[string]interface{})
	json_map = helpers.FlattenJson("", facterOutput, json_map)

	facts = buildFilteredMap(helpers.SanitizeJson(json_map), ignored_facts)

	return
}

func (self *FacterClient) runFacter() ([]byte, error) {
	facter_path, err := exec.LookPath("facter")
	if err != nil {
		return nil, err
	}

	path_env := "PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin"

	cmd := helpers.NewCommand(facter_path, "-p", "-j")
	if self.homeDir != "" {
		cmd.Cmd.Env = append(cmd.Cmd.Env, fmt.Sprintf("HOME=%s", self.homeDir))
	}
	cmd.Cmd.Env = append(cmd.Cmd.Env, path_env)
	output, err := cmd.Output()
	return output, err
}

func buildFilteredMap(facterJson map[string]interface{}, ignored_facts []string) map[string]FacterItem {
	filter := func(fact string) bool {
		for _, v := range ignored_facts {
			if strings.Contains(fact, v) {
				return true
			}
		}
		return false
	}
	//convert json to FacterItems so we can implement Id() for sorting
	facterItems := make(map[string]FacterItem)
	for k, v := range facterJson {
		if !filter(k) {
			facterItems[k] = FacterItem{k, v}
		}
	}
	return facterItems
}
