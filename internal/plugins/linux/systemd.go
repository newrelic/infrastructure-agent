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

var sdlog = log.WithPlugin("Systemd")
var reService = regexp.MustCompile(`(\S+)\.service.*loaded.*?(active|inactive).*?(running|failed|dead)(.*)`)
var systemdPluginId = ids.PluginID{"services", "systemd"}

type SystemdPlugin struct {
	agent.PluginCommon
	runningServices map[string]SystemdService
	frequency       time.Duration
}

type SystemdService struct {
	Name string `json:"id"`
	Pid  string `json:"pid"`
}

func (self SystemdService) SortKey() string {
	return self.Name
}

func (self SystemdPlugin) getSystemdDataset() agent.PluginInventoryDataset {
	var dataset agent.PluginInventoryDataset

	for _, v := range self.runningServices {
		dataset = append(dataset, v)
	}

	return dataset
}

func (self SystemdPlugin) getSystemdPidMap() map[int]string {
	result := make(map[int]string)

	for _, v := range self.runningServices {
		pid, err := strconv.Atoi(v.Pid)
		if err == nil {
			result[pid] = v.Name
		}
	}

	return result
}

func parseSystemctlOutput(line string) (name string, loaded string, status string) {
	matches := reService.FindStringSubmatch(line)
	if matches == nil {
		return "", "", ""
	}
	if len(matches) == 5 {
		name = matches[1]
		loaded = matches[2]
		status = matches[3]
		return name, loaded, status
	} else {
		sdlog.WithField("line", line).Warn("Saw unexpected line from systemctl")
		return "", "", ""
	}
}

func getPidFromName(line string) string {
	var pid string
	if pid_slice := strings.Split(line, "="); len(pid_slice) == 2 {
		pid = pid_slice[1]
		if pid == "0" {
			pid = "unknown"
		}
		if pid == "" {
			pid = "unknown"
		}
	} else {
		pid = "unknown"
	}
	return pid
}

func (self *SystemdPlugin) getSystemdServiceStatus() {
	output, err := helpers.RunCommand("/bin/systemctl", "", "--plain", "-l", "--no-pager", "--no-legend", "--all", "--type=service", "list-units")
	if err != nil {
		sdlog.WithError(err).Error("unable to get systemd service status")
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		// first up parse out the service name and status
		name, loaded, status := parseSystemctlOutput(scanner.Text())
		if name == "" {
			continue
		}
		if loaded == "inactive" {
			delete(self.runningServices, name)
			continue
		}
		if loaded == "active" {
			switch status {
			case "running":
				output, err := helpers.RunCommand("/bin/systemctl", "", "--property=MainPID", "show", name)
				if err != nil {
					sdlog.WithError(err).Error("systemctl unable to get pid")
				}
				pid := getPidFromName(output)
				self.runningServices[name] = SystemdService{name, pid}
			case "dead":
				delete(self.runningServices, name)
			case "exited":
				continue
			}
		}
	}
}

func systemdPresent() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func NewSystemdPlugin(ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &SystemdPlugin{
		PluginCommon:    agent.PluginCommon{ID: systemdPluginId, Context: ctx},
		runningServices: make(map[string]SystemdService),
		frequency: config.ValidateConfigFrequencySetting(
			cfg.SystemdIntervalSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_SYSTEMD_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self *SystemdPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		sdlog.Debug("Disabled.")
		return
	}

	if systemdPresent() {
		refreshTimer := time.NewTicker(1)
		for {
			select {
			case <-refreshTimer.C:
				{
					refreshTimer.Stop()
					refreshTimer = time.NewTicker(self.frequency)
					self.getSystemdServiceStatus()
					self.EmitInventory(self.getSystemdDataset(), self.Context.AgentIdentifier())
					self.Context.CacheServicePids(sysinfo.PROCESS_NAME_SOURCE_SYSTEMD, self.getSystemdPidMap())
				}
			}
		}
	} else {
		self.Unregister()
	}
}
