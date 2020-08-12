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
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

const (
	DPKG_INFO_DIR   = "/var/lib/dpkg/info"
	FSN_CLOSE_WRITE = 16
)

var dpkglog = log.WithPlugin("Dpkg")

type DpkgPlugin struct {
	agent.PluginCommon
	frequency time.Duration
}

type DpkgItem struct {
	Name         string `json:"id"`
	Architecture string `json:"architecture"`
	Essential    string `json:"essential"`
	Priority     string `json:"priority"`
	Status       string `json:"status"`
	Version      string `json:"version"`
	InstallTime  string `json:"installed_epoch"`
}

func (self DpkgItem) SortKey() string {
	return self.Name
}

func NewDpkgPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &DpkgPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.DpkgRefreshSec,
			config.FREQ_MINIMUM_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_PACKAGE_MGRS_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

// guessInstallTime this function makes a best guess at a package's install
// time by grabbing the creation time of the log of installed files for that
// package. For no clearly understandable reason the filename can be either
// just the package name OR the package_name suffixed with a :architecture
// we try for both and return a value or an empty string if we can't work it
// out at all
func (self *DpkgPlugin) guessInstallTime(packageName string, arch string) string {
	infoFile := filepath.Join(DPKG_INFO_DIR, fmt.Sprintf("%s.list", packageName))
	altFile := filepath.Join(DPKG_INFO_DIR, fmt.Sprintf("%s:%s.list", packageName, arch))

	fi, err := os.Stat(infoFile)
	if err != nil {
		// test the alternate file
		fi, err = os.Stat(altFile)
		if err != nil {
			return ""
		}
	}

	// For some strange reason, os.Stat() in golang doesn't return all the
	// items in the inode struct, particularly creation time (ctime).
	// Work around it by grabbing the fields out of the raw data returned
	// by the underlying system call.
	stat := fi.Sys().(*syscall.Stat_t)
	ctime := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))

	return fmt.Sprintf("%d", ctime.Unix())
}

func (self *DpkgPlugin) fetchPackageInfo() (packages agent.PluginInventoryDataset, err error) {
	output, err := helpers.RunCommand("/usr/bin/dpkg-query", "", "-W", "-f=${Package} ${Status} ${Architecture} ${Version} ${Essential} ${Priority}\n")
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 8 {
			continue
		}

		dpkgItem := DpkgItem{
			Name:         parts[0],
			Status:       parts[3],
			Architecture: parts[4],
			Version:      parts[5],
			Essential:    parts[6],
			Priority:     parts[7],
			InstallTime:  self.guessInstallTime(parts[0], parts[4]),
		}

		packages = append(packages, dpkgItem)
	}

	return
}

// Run is the main processing loop that drives the logic for the plugin
func (self *DpkgPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		dpkglog.Debug("Disabled.")
		return
	}

	// Subscribe to filesystem events are care about
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		dpkglog.WithError(err).Error("can't instantiate dpkg watcher")
		self.Unregister()
		return
	}

	err = watcher.Add("/var/lib/dpkg/lock")
	if err != nil {
		dpkglog.WithError(err).Error("can't setup trigger file watcher for dpkg")
		self.Unregister()
		return
	}

	counter := 1
	ticker := time.NewTicker(1)
	for {
		select {
		case event, ok := <-watcher.Events:
			if ok {
				if event.Op&fsnotify.Write == fsnotify.Write {
					counter = counter + 1
					if counter > 1 {
						dpkglog.WithFields(logrus.Fields{
							"frequency": self.frequency,
							"counter":   counter,
						}).Debug("dpkg plugin oversampling.")
					}
				}
			} else {
				dpkglog.Debug("dpkg lock watcher closed.")
				return
			}
		case <-ticker.C:
			ticker.Stop()
			ticker = time.NewTicker(self.frequency)
			if counter > 0 {
				data, err := self.fetchPackageInfo()
				if err != nil {
					dpkglog.WithError(err).Error("fetching dpkg data")
				} else {
					self.EmitInventory(data, self.Context.AgentIdentifier())
				}
				counter = 0
			}
		}
	}
}
