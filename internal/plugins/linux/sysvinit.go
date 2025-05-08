// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

const (
	SYSV_INIT_DIR = "/var/run"
)

var svlog = log.WithPlugin("Sysv")

type SysvInitPlugin struct {
	agent.PluginCommon
	frequency time.Duration
}

type SysvService struct {
	Name string `json:"id"`
	helpers.ServiceDetails
}

func (self SysvService) SortKey() string {
	return self.Name
}

func NewSysvInitPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &SysvInitPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.SysvInitIntervalSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_SYSVINIT_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self *SysvInitPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		svlog.Debug("Disabled.")
		return
	}

	first := true
	for {
		if first {
			first = false
		} else {
			time.Sleep(self.frequency)
		}

		dataset := types.PluginInventoryDataset{}
		pidMap := make(map[int]string)

		a, err := self.services(SYSV_INIT_DIR)
		if err != nil {
			svlog.WithError(err).WithField("syvDirectory", SYSV_INIT_DIR).Error("sysvinit reading pids")
			continue
		}
		for _, v := range a {
			dataset = append(dataset, v)
			pid, err := strconv.Atoi(v.ServiceDetails.Pid)
			if err == nil {
				pidMap[pid] = v.Name
			}
		}

		self.EmitInventory(dataset, entity.NewFromNameWithoutID(self.Context.EntityKey()))
		self.Context.CacheServicePids(sysinfo.PROCESS_NAME_SOURCE_SYSVINIT, pidMap)
	}
}

// we're verifying that the pidfile is not stale - if the process was
// started after the pidfile was written, then the pidfile is most
// likely stale we give us a little safety factor in case process
// uptime calc getGetPidDetails is a little faulty
func pidFileIsStale(pidFileMod, processStartTime time.Time) bool {
	return processStartTime.Sub(pidFileMod) > 5*time.Second
}

func (self *SysvInitPlugin) services(dirname string) ([]*SysvService, error) {
	// Find all *.pid files within /var/run (and follow symlinks)
	// Max depth of 3 ensures that, while we should catch pidfiles stored in a reasonable place, we
	// won't end up recursing into some kind of link to another filesystem hiding in /var/run.
	// (See MTBLS-683 for details on this issue)
	findCmd := exec.Command("find", "-L", dirname, "-maxdepth", "3", "-name", "*.pid")
	output, err := findCmd.Output()
	if err != nil {
		// This will get exit 1 if there were any files in /var/run which find refused to recurse into.
		// Unfortunately, there's not a deterministic way to detect this type of failure as opposed to
		// a more severe problem, so we basically just need to try to parse what we got.
	}

	a := []*SysvService{}
	outputReader := bytes.NewReader(output)
	outputScanner := bufio.NewScanner(outputReader)
	for outputScanner.Scan() {
		pidFile := outputScanner.Text()

		pidStat, err := os.Stat(pidFile)
		if err != nil {
			svlog.WithError(err).WithField("pidFile", pidFile).Error("sysvinit reading pid file")
			continue
		}

		if pidStat.IsDir() {
			// In case we get a .pid file which is actually a directory, we should ignore it.
			continue
		}

		pidFileMod := pidStat.ModTime()
		data, err := ioutil.ReadFile(pidFile)
		if err != nil {
			svlog.WithError(err).WithField("pidFile", pidFile).Error("sysvinit reading pid file")
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}
		svcDetails, err := helpers.GetPidDetails(pid)
		if err != nil {
			if !os.IsNotExist(err) {
				svlog.WithError(err).WithField("pidFile", pidFile).Debug("can't fetch process details for pidfile")
			}
			continue
		}
		// ignore processes not running with pid 1 as their parent, because they are
		// probably not actually services
		if svcDetails.Ppid != "1" {
			continue
		}
		if pidFileIsStale(pidFileMod, svcDetails.Started) {
			continue
		}
		svc := &SysvService{
			Name:           strings.TrimSuffix(filepath.Base(pidFile), ".pid"),
			ServiceDetails: svcDetails,
		}
		a = append(a, svc)
	}
	if outputScanner.Err() != nil {
		return nil, fmt.Errorf("Error scanning output of find: %v", err)
	}
	return a, nil
}
