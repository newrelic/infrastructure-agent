// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"bufio"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var ulog = log.WithPlugin("Upstart")

type UpstartPlugin struct {
	agent.PluginCommon
	runningServices map[string]UpstartService
	frequency       time.Duration
}

type UpstartService struct {
	Name string `json:"id"`
	Pid  string `json:"pid"`
}

func (self UpstartService) SortKey() string {
	return self.Name
}

func (self UpstartPlugin) getUpstartDataset() agent.PluginInventoryDataset {
	var dataset agent.PluginInventoryDataset

	for _, v := range self.runningServices {
		dataset = append(dataset, v)
	}

	return dataset
}

func (self UpstartPlugin) getUpstartPidMap() map[int]string {
	result := make(map[int]string)

	for _, v := range self.runningServices {
		pid, err := strconv.Atoi(v.Pid)
		if err == nil {
			result[pid] = v.Name
		}
	}

	return result
}

func (self *UpstartPlugin) getUpstartServiceStatus() {
	output, err := helpers.RunCommand("/sbin/initctl", "", "list")
	if err != nil {
		ulog.WithError(err).Error("unable to get upstart service status")
	}

	reService := regexp.MustCompile(`(\S+).*?(start|stop)\/(.*)`)
	rePid := regexp.MustCompile(`process\s+(\d+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		// first up parse out the service name and status
		line := scanner.Text()
		matches := reService.FindStringSubmatch(line)

		var name, status, rest string

		if len(matches) == 4 {
			name = matches[1]
			status = matches[2]
			rest = matches[3]
		} else {
			ulog.WithField("line", line).Warn("unexpected line from initctl")
		}

		var pid = "unknown"

		// try and extract a pid if it is listed
		matches = rePid.FindStringSubmatch(rest)
		if len(matches) == 2 {
			pid = matches[1]
		}

		// based on status add or remove service from state map
		switch status {
		case "start":
			self.runningServices[name] = UpstartService{name, pid}
		case "stop":
			delete(self.runningServices, name)
		}
	}
}

func upstartPresentAndWorking() bool {
	_, err := exec.LookPath("initctl")
	_, err = helpers.RunCommand("/sbin/initctl", "", "list")
	if err != nil {
		ulog.WithError(err).Debug("Can't find Upstart.")
	}
	return err == nil
}

func NewUpstartPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &UpstartPlugin{
		PluginCommon:    agent.PluginCommon{ID: id, Context: ctx},
		runningServices: make(map[string]UpstartService),
		frequency: config.ValidateConfigFrequencySetting(
			cfg.UpstartIntervalSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_UPSTART_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self *UpstartPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		ulog.Debug("Disabled.")
		return
	}

	if upstartPresentAndWorking() {
		refreshTimer := time.NewTicker(1)
		for {
			select {
			case <-refreshTimer.C:
				{
					refreshTimer.Stop()
					refreshTimer = time.NewTicker(self.frequency)
					self.getUpstartServiceStatus()
					self.EmitInventory(self.getUpstartDataset(), self.Context.AgentIdentifier())
					self.Context.CacheServicePids(sysinfo.PROCESS_NAME_SOURCE_UPSTART, self.getUpstartPidMap())
				}
			}
		}
	} else {
		self.Unregister()
	}
}
