// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/godbus/dbus/v5"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var usrlog = log.WithPlugin("Users")

const utmpPath = "/var/run/utmp"

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
func (self UsersPlugin) getUserDetails() (dataset types.PluginInventoryDataset) {
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

	// Strategy 1: Attempt to use the legacy utmp file watcher.
	// We check if the file exists first.
	if _, err := os.Stat(utmpPath); err == nil {
		usrlog.Debug("utmp file found, using legacy file watcher. path " + utmpPath)
		self.runUtmpWatcher()
	} else {
		// Strategy 2: Fallback to the modern D-Bus watcher for systemd-logind.
		usrlog.Debug("utmp file not found, falling back to modern D-Bus watcher for systemd-logind.")
		self.runDbusWatcher()
	}
}

// This function contains the original logic for watching /var/run/utmp.
func (self *UsersPlugin) runUtmpWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		usrlog.WithError(err).Error("can't instantiate legacy users watcher (fsnotify)")
		self.Unregister()
		return
	}
	defer watcher.Close()

	err = watcher.Add(utmpPath)
	if err != nil {
		usrlog.WithError(err).Error("can't setup trigger file watcher for users")
		self.Unregister()
		return
	}
	refreshTimer := time.NewTimer(1)
	needsFlush := true

	for {
		select {
		case event, ok := <-watcher.Events:
			if ok {
				if event.Op&fsnotify.Write == fsnotify.Write {
					needsFlush = true
				}
			}
		case <-refreshTimer.C:
			{
				refreshTimer.Reset(self.frequency)
				if needsFlush {
					self.EmitInventory(self.getUserDetails(), entity.NewFromNameWithoutID(self.Context.EntityKey()))
					needsFlush = false
				}
			}
		}
	}
}

// This function contains the logic for listening to systemd-logind signals via D-Bus.
func (self *UsersPlugin) runDbusWatcher() {
	conn, err := dbus.SystemBus()
	if err != nil {
		usrlog.WithError(err).Error("can't connect to system D-Bus, cannot monitor user sessions")
		self.Unregister()
		return
	}
	defer conn.Close()

	// D-Bus "match rules" for login and logout signals.
	rules := []string{
		"type='signal',interface='org.freedesktop.login1.Manager',member='SessionNew'",
		"type='signal',interface='org.freedesktop.login1.Manager',member='SessionRemoved'",
	}
	for _, rule := range rules {
		if err = conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err; err != nil {
			usrlog.WithError(err).Errorf("failed to add D-Bus match rule '%s'", rule)
			self.Unregister()
			return
		}
	}

	signals := make(chan *dbus.Signal, 10)
	conn.Signal(signals)

	refreshTimer := time.NewTimer(1)
	needsFlush := true

	for {
		select {
		case signal, ok := <-signals:
			if ok {
				if signal != nil {
					// Any signal (SessionNew or SessionRemoved) triggers a refresh.
					needsFlush = true
				}
			}
		case <-refreshTimer.C:
			{
				refreshTimer.Reset(self.frequency)
				if needsFlush {
					self.EmitInventory(self.getUserDetails(), entity.NewFromNameWithoutID(self.Context.EntityKey()))
					needsFlush = false
				}
			}
		}
	}
}
