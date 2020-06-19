// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package linux

import (
	"bufio"
	"github.com/fsnotify/fsnotify"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var usrlog = log.WithPlugin("Users")

type UsersPlugin struct {
	agent.PluginCommon
	frequency time.Duration
}

type User struct {
	Name string `json:"id"`
}

func (self User) SortKey() string {
	return self.Name
}

var usersPluginID = ids.PluginID{"sessions", "users"}

func NewUsersPlugin(ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &UsersPlugin{
		PluginCommon: agent.PluginCommon{ID: usersPluginID, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.UsersRefreshSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_USERS_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

// getUserDetails runs the who command, parses it's output and returns
// a dataset of users.
func (self UsersPlugin) getUserDetails() (dataset agent.PluginInventoryDataset) {
	output, err := helpers.RunCommand("/usr/bin/env", "", "who")
	if err != nil {
		usrlog.WithError(err).Error("failed to fetch user information")
		return
	}
	users := parseWhoOutput(output)
	for k := range users {
		dataset = append(dataset, User{
			Name: k,
		})
	}

	return dataset
}

// parseWhoOutput parses the output from an execution of the `who`
// command. Returns a map that serves as a set in which the keys
// are the users received in the `who` output.
func parseWhoOutput(output string) (users map[string]bool) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	users = make(map[string]bool)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			usrlog.Warn("empty line in who output")
			continue
		}
		user := fields[0]
		users[user] = true
	}
	return users
}

func (self *UsersPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		usrlog.Debug("Disabled.")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		usrlog.WithError(err).Error("can't instantiate users watcher")
		self.Unregister()
		return
	}

	err = watcher.WatchFlags("/var/run/utmp", fsnotify.FSN_MODIFY)
	if err != nil {
		usrlog.WithError(err).Error("can't setup trigger file watcher for users")
		self.Unregister()
		return
	}
	refreshTimer := time.NewTimer(1)
	needsFlush := true

	for {
		select {
		case <-watcher.Event:
			needsFlush = true
		case <-refreshTimer.C:
			{
				refreshTimer.Reset(self.frequency)
				if needsFlush {
					self.EmitInventory(self.getUserDetails(), self.Context.AgentIdentifier())
					needsFlush = false
				}
			}
		}
	}
}
