// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"bufio"
	"fmt"
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
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/shirou/gopsutil/host"
	"github.com/sirupsen/logrus"
)

var hlog = log.WithComponent("HostInfoPlugin")

type HostinfoPlugin struct {
	agent.PluginCommon
	cloudHarvester cloud.Harvester // Gather metadata for the cloud instance.
}

type HostinfoData struct {
	System        string `json:"id"`
	Distro        string `json:"distro"`
	KernelVersion string `json:"kernel_version"`
	HostType      string `json:"host_type"`
	CpuName       string `json:"cpu_name"`
	// Number of cores within a single CPU ('cpu cores' field in /proc/cpuinfo)
	// It is shown as 'coreCount' in New Relic UI
	CpuNum string `json:"cpu_num"`
	// Total number of cores in all the CPUs ('processor' entries in /proc/cpuinfo)
	// It is shown as 'processorCount' in New Relic UI
	TotalCpu        string `json:"total_cpu"`
	Ram             string `json:"ram"`
	UpSince         string `json:"boot_timestamp"`
	AgentVersion    string `json:"agent_version"`
	AgentName       string `json:"agent_name"`
	AgentMode       string `json:"agent_mode"`
	OperatingSystem string `json:"operating_system"`
	ProductUuid     string `json:"product_uuid"`
	BootId          string `json:"boot_id"`
	RegionAWS       string `json:"aws_region,omitempty"`
	RegionAzure     string `json:"region_name,omitempty"`
	RegionGCP       string `json:"zone,omitempty"`
	RegionAlibaba   string `json:"region_id,omitempty"`
}

func (self HostinfoData) SortKey() string {
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

func NewHostinfoPlugin(ctx agent.AgentContext, cloudHarvester cloud.Harvester) agent.Plugin {
	return &HostinfoPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.HostInfo,
			Context: ctx,
		},
		cloudHarvester: cloudHarvester,
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

func (self *HostinfoPlugin) gatherHostinfo(context agent.AgentContext) *HostinfoData {
	infoFile := helpers.HostProc("/cpuinfo")
	totalCpu := getTotalCpu(infoFile)
	var productUuid string
	var hostType string
	if distro.IsCentos5() {
		productUuid = "unknown"
		hostType = "unknown unknown"
	} else {
		productUuid = getProductUuid(context.Config().RunMode)
		hostType = self.getHostType()
	}

	data := &HostinfoData{
		System:          "system",
		Distro:          distro.GetDistro(),
		KernelVersion:   getKernelRelease(),
		HostType:        hostType,
		CpuName:         readProcFile(helpers.HostProc("/cpuinfo"), regexp.MustCompile(`model\sname\s*:\s`)),
		CpuNum:          getCpuNum(infoFile, totalCpu),
		TotalCpu:        totalCpu,
		Ram:             readProcFile(helpers.HostProc("/meminfo"), regexp.MustCompile(`MemTotal:\s*`)),
		UpSince:         getUpSince(),
		AgentVersion:    context.Version(),
		AgentName:       "Infrastructure",
		AgentMode:       string(context.Config().RunMode),
		OperatingSystem: runtime.GOOS,
		ProductUuid:     productUuid,
		BootId:          fingerprint.GetBootId(),
	}
	err := self.setCloudRegion(data)
	if err != nil {
		hlog.WithError(err).WithField("cloudType", self.cloudHarvester.GetCloudType()).Debug(
			"cloud region couldn't be set")
	}
	helpers.LogStructureDetails(hlog, data, "HostInfoData", "raw", nil)

	return data
}

func (self *HostinfoPlugin) setCloudRegion(data *HostinfoData) (err error) {
	if self.Context.Config().DisableCloudMetadata ||
		self.cloudHarvester.GetCloudType() == cloud.TypeNoCloud {
		return
	}

	region, err := self.cloudHarvester.GetRegion()
	if err != nil {
		return fmt.Errorf("couldn't retrieve cloud region: %v", err)
	}

	switch self.cloudHarvester.GetCloudType() {
	case cloud.TypeAWS:
		data.RegionAWS = region
	case cloud.TypeAzure:
		data.RegionAzure = region
	case cloud.TypeGCP:
		data.RegionGCP = region
	case cloud.TypeAlibaba:
		data.RegionAlibaba = region
	default:
	}
	return
}

func (self *HostinfoPlugin) getHostType() string {
	if self.Context.Config().DisableCloudMetadata ||
		self.cloudHarvester.GetCloudType() == cloud.TypeNoCloud {

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

	hostType := "unknown"
	if response, err := self.cloudHarvester.GetHostType(); err != nil {
		hlog.WithError(err).WithField("cloudType", self.cloudHarvester.GetCloudType()).Debug(
			"error getting host type from cloud metadata")
	} else {
		hostType = response
	}

	return hostType
}
