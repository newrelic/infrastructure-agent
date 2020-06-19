// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

// File Monitor Plugin
// Watches for changes to files in the interesting directories
package linux

import (
	"bufio"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"
	"sort"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var rpmlog = log.WithPlugin("Rpm")

// Paths
const (
	RpmPath = "/bin/rpm"
)

type rpmPlugin struct {
	agent.PluginCommon
	frequency    time.Duration
	erroredLines map[string]struct{}
}

type RpmItem struct {
	Name         string `json:"id"`
	Version      string `json:"version"`
	Release      string `json:"release"`
	Architecture string `json:"architecture"`
	InstallTime  string `json:"installed_epoch"`
	EpochTag     string `json:"epoch_tag"`
}

func (p RpmItem) SortKey() string {
	return p.Name
}

var pluginId = ids.PluginID{"packages", "rpm"}

func NewRpmPlugin(ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &rpmPlugin{
		PluginCommon: agent.PluginCommon{ID: pluginId, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.RpmRefreshSec,
			config.FREQ_MINIMUM_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_PACKAGE_MGRS_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
		erroredLines: make(map[string]struct{}),
	}
}

func (p *rpmPlugin) fetchPackageInfo() (packages agent.PluginInventoryDataset, err error) {
	output, err := helpers.RunCommand(RpmPath, "", "-qa", "--queryformat=%{NAME} %{VERSION} %{RELEASE} %{ARCH} %{INSTALLTIME} %{EPOCH}\n")
	if err != nil {
		return nil, err
	}
	return p.parsePackageInfo(output)
}

func (p *rpmPlugin) parsePackageInfo(output string) (packages agent.PluginInventoryDataset, err error) {
	// Get output and sort it alphabetically to ensure consistent ordering
	var outputLines []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		outputLines = append(outputLines, scanner.Text())
	}
	sort.Strings(outputLines)

	nameDuplicateTracker := make(map[string]int)
	for _, outputLine := range outputLines {
		parts := strings.Fields(outputLine)
		if len(parts) < 6 {
			if _, exists := p.erroredLines[outputLine]; !exists {
				p.erroredLines[outputLine] = struct{}{}
				rpmlog.WithField("outputLine", outputLine).Warn("cannot parse rpm query line")
			}
			continue
		}

		// If we've hit this package name before, append -1, -2, etc to handle duplicates.
		name := parts[0]
		nameCount, _ := nameDuplicateTracker[name]
		nameDuplicateTracker[name] = nameCount + 1
		if nameCount > 0 {
			name = fmt.Sprintf("%v-%v", name, nameCount)
		}

		epoch := parts[5]
		if strings.Contains(epoch, "none") {
			epoch = "none"
		}

		RpmItem := RpmItem{
			Name:         name,
			Version:      parts[1],
			Release:      parts[2],
			Architecture: parts[3],
			InstallTime:  parts[4],
			EpochTag:     epoch,
		}

		packages = append(packages, RpmItem)
	}

	return
}

// Run is the main processing loop that drives the logic for the plugin
func (p *rpmPlugin) Run() {
	if p.frequency <= config.FREQ_DISABLE_SAMPLING {
		rpmlog.Debug("Disabled.")
		return
	}

	// Subscribe to filesystem events are care about
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		rpmlog.WithError(err).Error("can't instantiate rpm watcher")
		p.Unregister()
		return
	}

	err = watcher.WatchFlags("/var/lib/rpm/.rpm.lock", FSN_CLOSE_WRITE)
	if err != nil {
		// Some old distros, like SLES 11, do not provide .rpm.lock file, but the same
		// effect can be achieved by listening some standard files from the RPM database
		err = watcher.WatchFlags("/var/lib/rpm/Installtid", FSN_CLOSE_WRITE)
		if err != nil {
			rpmlog.WithError(err).Error("can't setup trigger file watcher for rpm")
			p.Unregister()
			return
		}
	}

	counter := 1
	ticker := time.NewTicker(1)
	for {
		select {
		case _, ok := <-watcher.Event:
			if ok {
				counter = counter + 1
				if counter > 1 {
					rpmlog.WithFields(logrus.Fields{
						"frequency": p.frequency,
						"counter":   counter,
					}).Debug("rpm plugin oversampling.")
				}
			} else {
				rpmlog.Debug("rpm lock watcher closed.")
				return
			}
		case <-ticker.C:
			ticker.Stop()
			ticker = time.NewTicker(p.frequency)
			if counter > 0 {
				data, err := p.fetchPackageInfo()
				if err != nil {
					rpmlog.WithError(err).Error("fetching rpm data")
				} else {
					p.EmitInventory(data, p.Context.AgentIdentifier())
				}
				counter = 0
			}
		}
	}
}
