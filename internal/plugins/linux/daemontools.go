// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var dtlog = log.WithPlugin("Daemontools")

type DaemontoolsService struct {
	Name string `json:"id"`
	Pid  string `json:"pid"`
}

func (self DaemontoolsService) SortKey() string {
	return self.Name
}

type DaemontoolsPlugin struct {
	agent.PluginCommon
	frequency time.Duration
}

func NewDaemontoolsPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &DaemontoolsPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.DaemontoolsRefreshSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_DAEMONTOOLS_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func daemonToolsPresent() bool {
	_, err := exec.LookPath("svscan")
	return err == nil
}

func getDaemontoolsServiceStatus() (data agent.PluginInventoryDataset, pidMap map[int]string, err error) {
	pidMap = make(map[int]string)
	var (
		psOut []byte
	)
	cmd := exec.Command("ps", "-e", "-o", "pid,args")
	psOut, err = cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Error grabbing process list: %s output: %s", err, string(psOut))
		return
	}
	scan := bufio.NewScanner(bytes.NewBuffer(psOut))
	for scan.Scan() {
		pid, svcName, err := parseSuperviseProcessListing(scan.Text())
		if err != nil {
			dtlog.WithError(err).Error("trying to parse supervise info out of process listing")
			continue
		}
		if pid == 0 {
			continue
		}
		svstatWd, err := os.Readlink(helpers.HostProc(strconv.FormatInt(pid, 10), "cwd"))
		if err != nil {
			dtlog.WithError(err).WithFields(logrus.Fields(logrus.Fields{
				"serviceName": svcName,
				"pid":         pid,
			})).Error("checking working directory of daemontools service")
		}
		svstat := exec.Command("svstat", svstatWd)
		svstatOut, err := svstat.CombinedOutput()
		if err != nil {
			dtlog.WithError(err).WithFields(logrus.Fields(logrus.Fields{
				"serviceName": svcName,
				"output":      string(svstatOut),
			})).Error("getting svstat")
			continue
		}
		up, pid, err := parseSvstatOutput(string(svstatOut))
		if err != nil {
			dtlog.WithError(err).WithFields(logrus.Fields(logrus.Fields{
				"serviceName": svcName,
				"output":      string(svstatOut),
			})).Error("parsing svstat")
			continue
		}
		if up {
			pidMap[int(pid)] = svcName
			data = append(data, DaemontoolsService{svcName, strconv.FormatInt(pid, 10)})
		}
	}

	return
}

var svstatPidRegexp = regexp.MustCompile(`up \(pid (\d+)\)`)
var psSuperviseLineRegexp = regexp.MustCompile(`^(\d+) supervise (.+)$`)

func parseSvstatOutput(output string) (up bool, pid int64, err error) {
	matches := svstatPidRegexp.FindStringSubmatch(output)
	if len(matches) == 0 {
		return
	}
	up = true
	pid, err = strconv.ParseInt(matches[1], 10, 64)
	return
}

func parseSuperviseProcessListing(psLine string) (pid int64, svcName string, err error) {
	matches := psSuperviseLineRegexp.FindStringSubmatch(strings.TrimSpace(psLine))
	if len(matches) == 0 {
		return
	}
	svcName = matches[2]
	pid, err = strconv.ParseInt(matches[1], 10, 64)
	return
}

func (self *DaemontoolsPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		dtlog.Debug("Disabled.")
		return
	}

	if daemonToolsPresent() {
		checkTimer := time.NewTicker(1)
		for {
			<-checkTimer.C
			checkTimer.Stop()
			checkTimer = time.NewTicker(self.frequency)
			services, pidMap, err := getDaemontoolsServiceStatus()
			if err == nil {
				self.EmitInventory(services, self.Context.AgentIdentifier())
				self.Context.CacheServicePids(sysinfo.PROCESS_NAME_SOURCE_DAEMONTOOLS, pidMap)
			} else {
				dtlog.WithError(err).Error("getting daemontools status")
			}
		}
	} else {
		self.Unregister()
	}
}
