//Copyright 2020 New Relic Corporation. All rights reserved.
//SPDX-License-Identifier: Apache-2.0
//+build darwin

package darwin

import (
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"runtime"
	"strconv"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/os/fs"
	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/shirou/gopsutil/host"
)

var hlog = log.WithComponent("HostInfoPlugin")

type HostInfoPlugin struct {
	agent.PluginCommon
	cloudHarvester cloud.Harvester // Gather metadata for the cloud instance.
}

type HostInfoData struct {
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

func (h HostInfoData) SortKey() string {
	return h.System
}

func getProductUuid(mode string) string {
	const unknownProductUUID = "unknown"

	return unknownProductUUID
}

func NewHostInfoPlugin(ctx agent.AgentContext, cloudHarvester cloud.Harvester) agent.Plugin {
	return &HostInfoPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.HostInfo,
			Context: ctx,
		},
		cloudHarvester: cloudHarvester,
	}
}

func (h *HostInfoPlugin) Data() agent.PluginInventoryDataset {
	return agent.PluginInventoryDataset{h.gatherHostInfo(h.Context)}
}

func (h *HostInfoPlugin) Run() {
	h.Context.AddReconnecting(h)
	data := h.Data()
	h.EmitInventory(data, entity.NewFromNameWithoutID(h.Context.EntityKey()))
}

func (h *HostInfoPlugin) gatherHostInfo(context agent.AgentContext) *HostInfoData {

	hostInfo := hostInfoStat()
	cpuInfo := cpuInfoStat()
	memoryInfo := memoryInfoStat()

	var productUuid string
	var hostType string

	productUuid = getProductUuid(context.Config().RunMode)
	hostType = h.getHostType()

	data := &HostInfoData{
		System:          "system",
		Distro:          hostInfo.PlatformVersion,
		KernelVersion:   hostInfo.KernelVersion,
		HostType:        hostType,
		CpuName:         cpuName(cpuInfo),
		CpuNum:          strconv.Itoa(int(coresCount(cpuInfo))),
		TotalCpu:        strconv.Itoa(len(cpuInfo)),
		Ram:             fmt.Sprintf("%v", memoryInfo.Total),
		UpSince:         time.Now().Add(time.Second * -time.Duration(hostInfo.Uptime)).Format("2006-01-02 15:04:05"),
		AgentVersion:    context.Version(),
		AgentName:       "Infrastructure",
		AgentMode:       context.Config().RunMode,
		OperatingSystem: runtime.GOOS,
		ProductUuid:     productUuid,
		BootId:          fingerprint.GetBootId(),
	}

	err := h.setCloudRegion(data)
	if err != nil {
		hlog.WithError(err).WithField("cloudType", h.cloudHarvester.GetCloudType()).Debug(
			"cloud region couldn't be set")
	}
	helpers.LogStructureDetails(hlog, data, "HostInfoData", "raw", nil)

	return data
}

func memoryInfoStat() *mem.VirtualMemoryStat {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return &mem.VirtualMemoryStat{}
	}
	return vm
}

func cpuName(info []cpu.InfoStat) string {
	for _, i := range info {
		return i.ModelName
	}
	return ""
}
func coresCount(info []cpu.InfoStat) int32 {
	var cores int32
	for _, i := range info {
		cores += i.Cores
	}
	return cores
}

func cpuInfoStat() []cpu.InfoStat {
	cpuInfo, err := cpu.Info()
	if err != nil {
		return []cpu.InfoStat{}
	}
	return cpuInfo
}

func hostInfoStat() *host.InfoStat {
	info, err := host.Info()
	if err != nil {
		info = &host.InfoStat{
			OS: runtime.GOOS,
		}
	}
	return info
}

func (h *HostInfoPlugin) setCloudRegion(data *HostInfoData) (err error) {
	if h.Context.Config().DisableCloudMetadata ||
		h.cloudHarvester.GetCloudType() == cloud.TypeNoCloud {
		return
	}

	region, err := h.cloudHarvester.GetRegion()
	if err != nil {
		return fmt.Errorf("couldn't retrieve cloud region: %v", err)
	}

	switch h.cloudHarvester.GetCloudType() {
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

func (h *HostInfoPlugin) getHostType() string {
	hostType := "unknown"

	if h.Context.Config().DisableCloudMetadata ||
		h.cloudHarvester.GetCloudType() == cloud.TypeNoCloud ||
		h.cloudHarvester.GetCloudType() == cloud.TypeInProgress {

		manufacturer, err := fs.ReadFirstLine(helpers.HostSys("/devices/virtual/dmi/id/sys_vendor"))
		if err != nil {
			hlog.WithError(err).Error("cannot read dmi sys vendor")
		}

		name, err := fs.ReadFirstLine(helpers.HostSys("/devices/virtual/dmi/id/product_name"))
		if err != nil {
			hlog.WithError(err).Error("cannot read dmi product name")
		}

		hostType = fmt.Sprintf("%s %s", manufacturer, name)
	}

	if response, err := h.cloudHarvester.GetHostType(); err != nil {
		hlog.WithError(err).WithField("cloudType", h.cloudHarvester.GetCloudType()).Debug(
			"error getting host type from cloud metadata")
	} else {
		hostType = response
	}

	return hostType
}
