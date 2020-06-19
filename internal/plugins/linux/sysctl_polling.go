// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/lru"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

type SysctlPlugin struct {
	agent.PluginCommon
	sysctls       agent.PluginInventoryDataset
	errorsLogged  map[string]bool
	frequency     time.Duration
	procSysDir    string
	fileService   fileService
	ignoredListRE *regexp.Regexp
	regexpCache   *lru.Cache
}

// NewSysctlPollingMonitor creates a /proc/sys parser polling on intervals
func NewSysctlPollingMonitor(id ids.PluginID, ctx agent.AgentContext) *SysctlPlugin {
	cfg := ctx.Config()
	return &SysctlPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		errorsLogged: make(map[string]bool),
		frequency: config.ValidateConfigFrequencySetting(
			cfg.SysctlIntervalSec,
			config.FREQ_MINIMUM_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_SYSCTL_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
		procSysDir: helpers.HostProc("/sys/"),
		fileService: fileService{
			walk: filepath.Walk,
			read: ioutil.ReadFile,
		},
		ignoredListRE: regexp.MustCompile(fmt.Sprintf("(%s)", strings.Join(ignoredListPatterns, ")|("))),
		regexpCache:   lru.New(),
	}
}

// walkSysctl will read the value of the /proc/sys item with some simple constraints:
//   1) the file must be writable (implying it can be changed)
//   2) the file must also be readable - there are some write only sysctls
//   3) directories under /proc/sys are never writable so no need to explicitly
//      check if path is a regular file or directory
// the walk function will also only log errors rather than causing the calling Walk
// to ever error out - we want to log and skip unreadables rather than abort the Walk
func (self *SysctlPlugin) walkSysctl(path string, fi os.FileInfo, _ error) (err error) {
	// if for some reason the file object is nil, bail
	if fi == nil {
		sclog.WithField("path", path).Debug("Systcl file is nil.")
		return
	}

	perm := fi.Mode().Perm()
	// if the file isn't writeable or file isn't readable bail
	if perm&WRITABLE_MASK == 0 || perm&READABLE_MASK == 0 {
		return
	}

	if fi.IsDir() {
		return
	}

	matches, ok := self.regexpCache.Get(path)
	if !ok {
		matches = self.ignoredListRE.MatchString(path)
		self.regexpCache.Add(path, matches)
	}
	if matches == true {
		return
	}

	output, readFileErr := self.fileService.read(path)
	if readFileErr != nil {
		if os.IsNotExist(readFileErr) {
			sclog.WithError(readFileErr).WithField("path", path).Error("error reading file")
			return
		}
		errMessage := fmt.Sprintf("Unable to read sysctl from %s, skipping: %s", path, readFileErr)
		if !self.errorsLogged[errMessage] {
			self.errorsLogged[errMessage] = true
			sclog.Error(errMessage)
		}
		return
	}

	self.sysctls = append(self.sysctls, self.newSysctlItem(path, output))
	return
}

// reformat path into sysctl style dot separated
func (self *SysctlPlugin) newSysctlItem(filePath string, output []byte) SysctlItem {
	keyPath := strings.TrimPrefix(filePath, self.procSysDir)
	keyPath = strings.Replace(keyPath, "/", ".", -1)
	return SysctlItem{keyPath, strings.TrimSpace(string(output))}
}

func (self *SysctlPlugin) Sysctls() (dataset agent.PluginInventoryDataset, err error) {
	// Clear out the list, since we're going to be repopulating it completely anyway and we want to drop any entries we don't find anymore.
	self.sysctls = make([]agent.Sortable, 0)

	if err := self.fileService.walk(self.procSysDir, self.walkSysctl); err != nil {
		return nil, err
	}

	// We remove old entries from the Regexp Cache (files that have been deleted since previous execution)
	self.regexpCache.RemoveUntilLen(len(self.sysctls))

	return self.sysctls, nil
}

// Run is where you implement your plugin logic
func (self *SysctlPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		sclog.Debug("Disabled.")
		return
	}

	ticker := time.NewTicker(1)
	for {
		select {
		case <-ticker.C:
			ticker.Stop()
			ticker = time.NewTicker(self.frequency)
			dataset, err := self.Sysctls()
			if err != nil {
				sclog.WithError(err).Error("fetching sysctl data")
			} else {
				self.EmitInventory(dataset, self.Context.AgentIdentifier())
			}
		}
	}
}
