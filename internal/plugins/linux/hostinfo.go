// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package linux

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/os/distro"
	"github.com/newrelic/infrastructure-agent/internal/os/fs"
	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/sirupsen/logrus"
)

var (
	hlog = log.WithComponent("HostInfoPlugin")
)

type HostinfoPlugin struct {
	agent.PluginCommon
	common.HostInfo
}

type HostInfoLinux struct {
	Distro              string `json:"distro"`
	KernelVersion       string `json:"kernel_version"`
	AgentMode           string `json:"agent_mode"`
	ProductUuid         string `json:"product_uuid"`
	BootId              string `json:"boot_id"`
	common.HostInfoData `mapstructure:",squash"`
}

func (self HostInfoLinux) SortKey() string {
	return self.System
}

func getTotalCpu(cpuInfoFile string) string {
	cpu_re := regexp.MustCompile(`processor\s*:\s*([0-9]+)`)
	file, err := ioutil.ReadFile(cpuInfoFile)
	if err != nil {
		hlog.WithError(err).Error("opening proc file /cpuinfo")
		return "unknown"
	}
	totalCpuNum := cpu_re.FindAllString(string(file), -1)
	return strconv.Itoa(len(totalCpuNum))
}

func getKernelRelease() string {
	v, err := fs.ReadFirstLine(helpers.HostProc("/sys/kernel/osrelease"))
	if err != nil {
		hlog.WithError(err).Error("error reading kernel release")
	}
	return v
}

func getProductUuid(mode string) string {
	const unknownProductUUID = "unknown"

	if mode == config.ModeUnprivileged {
		return unknownProductUUID
	}

	uuid, err := fs.ReadFirstLine(helpers.HostSys("/class/dmi/id/product_uuid"))
	if err != nil {
		hlog.WithError(err).Error("error reading uuid")
	}
	matched, err := regexp.MatchString("^[0-9A-F]{8}(-[0-9A-F]{4}){3}-[0-9A-F]{12}$", uuid)
	if err != nil {
		hlog.WithError(err).Error("error in checking regular expression")
		return unknownProductUUID
	}

	if matched == false {
		hlog.WithFields(logrus.Fields{
			"UUID": uuid,
		}).Debug("Unexpected format for product uuid.")
		return unknownProductUUID
	}

	return uuid
}

func readProcFile(filename string, re *regexp.Regexp) string {
	file, err := os.Open(filename)
	if err != nil {
		hlog.WithError(err).WithField("filename", filename).Error("opening proc file")
		return "unknown"
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := re.Split(scanner.Text(), -1); len(line) == 2 {
			return line[1]
		}
	}
	if err := scanner.Err(); err != nil {
		hlog.WithError(err).WithField("filename", filename).Error("reading proc file")
	}
	return "unknown"
}

func getUpSince() string {
	info, err := host.Info()
	if err != nil {
		hlog.WithError(err).Warn("unable to read host info for uptime")
		return ""
	}
	return time.Now().Add(time.Second * -time.Duration(info.Uptime)).Format("2006-01-02 15:04:05")
}

func runCmd(command string, args ...string) string {
	cmd := helpers.NewCommand(command, args...)
	output, err := cmd.Output()
	if err != nil {
		hlog.WithError(err).WithField("command", command).Debug("Executing command.")
		return "unknown"
	}
	return string(output)
}

func NewHostinfoPlugin(ctx agent.AgentContext, hostInfo common.HostInfo) agent.Plugin {
	return &HostinfoPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.HostInfo,
			Context: ctx,
		},
		HostInfo: hostInfo,
	}
}

func (self *HostinfoPlugin) Data() agent.PluginInventoryDataset {
	return agent.PluginInventoryDataset{self.gatherHostinfo(self.Context)}
}

func (self *HostinfoPlugin) Run() {
	self.Context.AddReconnecting(self)
	data := self.Data()
	self.EmitInventory(data, entity.NewFromNameWithoutID(self.Context.EntityKey()))
}

// getCpuNum reads the `cpu cores` entry from `cpuInfoFile`, on some
// SUSE versions when the host has just one cpu this entry is missing,
// in that case you can specify a fallback value to return.
func getCpuNum(cpuInfoFile string, fallback string) string {
	cpuCores := readProcFile(cpuInfoFile, regexp.MustCompile(`cpu\scores\s*:\s`))
	if cpuCores == "unknown" {
		return fallback
	} else {
		return cpuCores
	}
}

func (self *HostinfoPlugin) gatherHostinfo(context agent.AgentContext) *HostInfoLinux {
	infoFile := helpers.HostProc("/cpuinfo")
	totalCpu := getTotalCpu(infoFile)
	var productUuid string
	if distro.IsCentos5() {
		productUuid = "unknown"
	} else {
		productUuid = getProductUuid(context.Config().RunMode)
	}

	commonHostInfo, err := self.GetHostInfo()
	if err != nil {
		hlog.WithError(err).Error("error fetching host data information")
	}

	data := &HostInfoLinux{
		HostInfoData:  commonHostInfo,
		Distro:        distro.GetDistro(),
		KernelVersion: getKernelRelease(),
		AgentMode:     string(context.Config().RunMode),
		ProductUuid:   productUuid,
		BootId:        fingerprint.GetBootId(),
	}

	// set specific OS fields
	if data.HostType, err = self.GetCloudHostType(); err != nil {
		if errors.Is(err, common.ErrNoCloudHostTypeNotAvailable) {
			data.HostType = self.getHostType()
		} else {
			hlog.WithError(err).Debug("error getting host type from cloud metadata")
		}
	}

	data.CpuName = readProcFile(helpers.HostProc("/cpuinfo"), regexp.MustCompile(`model\sname\s*:\s`))
	data.CpuNum = getCpuNum(infoFile, totalCpu)
	data.TotalCpu = totalCpu
	data.Ram = readProcFile(helpers.HostProc("/meminfo"), regexp.MustCompile(`MemTotal:\s*`))
	data.UpSince = getUpSince()
	data.OperatingSystem = runtime.GOOS

	helpers.LogStructureDetails(hlog, data, "HostInfoData", "raw", nil)

	return data
}

func (self *HostinfoPlugin) getHostType() string {
	manufacturer, err := fs.ReadFirstLine(helpers.HostSys("/devices/virtual/dmi/id/sys_vendor"))
	if err != nil {
		hlog.WithError(err).Error("cannot read dmi sys vendor")
	}

	name, err := fs.ReadFirstLine(helpers.HostSys("/devices/virtual/dmi/id/product_name"))
	if err != nil {
		hlog.WithError(err).Error("cannot read dmi product name")
	}

	return fmt.Sprintf("%s %s", manufacturer, name)
}
