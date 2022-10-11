// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build freebsd
// +build freebsd

package freebsd

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	hlog               = log.WithComponent("HostInfoPlugin")
	kernelVersionRegex = regexp.MustCompile("Version (.*?):")
)

// HostinfoPlugin returns metadata of the host.
type HostinfoPlugin struct {
	agent.PluginCommon
	cloudHarvester cloud.Harvester // Gather metadata for the cloud instance.

	readDataFromCmd func(cmd string, args ...string) (string, error)
}

// HostInfoData is the structure returned to the backend by HostinfoPlugin.
type HostInfoData struct {
	System        string `json:"id"`
	Distro        string `json:"distro"`
	KernelVersion string `json:"kernel_version"`
	HostType      string `json:"host_type"`
	CpuName       string `json:"cpu_name"`
	// Number of cores within a single CPU
	// It is shown as 'coreCount' in New Relic UI
	CpuNum string `json:"cpu_num"`
	// Total number of cores in all the CPUs
	// It is shown as 'processorCount' in New Relic UI
	TotalCpu            string `json:"total_cpu"`
	Ram                 string `json:"ram"`
	UpSince             string `json:"boot_timestamp"`
	AgentVersion        string `json:"agent_version"`
	AgentName           string `json:"agent_name"`
	AgentMode           string `json:"agent_mode"`
	OperatingSystem     string `json:"operating_system"`
	ProductUuid         string `json:"product_uuid"`
	RegionAWS           string `json:"aws_region,omitempty"`
	RegionAzure         string `json:"region_name,omitempty"`
	RegionGCP           string `json:"zone,omitempty"`
	RegionAlibaba       string `json:"region_id,omitempty"`
	AWSAccountID        string `json:"aws_account_id,omitempty"`
	AWSAvailabilityZone string `json:"aws_availability_zone,omitempty"`
	AWSImageID          string `json:"aws_image_id,omitempty"`
}

func (hip *HostInfoData) SortKey() string {
	return hip.System
}

func NewHostinfoPlugin(ctx agent.AgentContext, cloudHarvester cloud.Harvester) agent.Plugin {
	return &HostinfoPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.HostInfo,
			Context: ctx,
		},
		cloudHarvester: cloudHarvester,
		readDataFromCmd: func(cmd string, args ...string) (string, error) {
			return helpers.RunCommand(cmd, "", args...)
		},
	}
}

// Run the HostinfoPlugin.
func (hip *HostinfoPlugin) Run() {
	hip.Context.AddReconnecting(hip)
	data := hip.Data()
	hip.EmitInventory(data, entity.NewFromNameWithoutID(hip.Context.EntityKey()))
}

// Data collects the hostinfo.
func (hip *HostinfoPlugin) Data() agent.PluginInventoryDataset {
	return agent.PluginInventoryDataset{hip.gatherHostinfo(hip.Context)}
}

func (hip *HostinfoPlugin) gatherHostinfo(context agent.AgentContext) *HostInfoData {
	ho, err := hip.getHardwareOverview()
	if err != nil {
		hlog.WithError(err).Error("error reading hardware overview")
	}

	kernelVersion, err := hip.getKernelRelease()
	if err != nil {
		hlog.WithError(err).Error("error reading kernel release")
	}

	totalNumberOfCores, err := strconv.Atoi(ho.TotalNumberOfCores)
	if err != nil {
		hlog.WithError(err).Error("error converting total number of cores to int")
	}

	cpuNum := 0
	if ho.NumberOfProcessors > 0 {
		cpuNum = totalNumberOfCores / ho.NumberOfProcessors
	}

	data := &HostInfoData{
		System:          "system",
		Distro:          hip.getOSDistribution(),
		KernelVersion:   kernelVersion,
		HostType:        hip.getHostType(ho),
		CpuName:         fmt.Sprintf("%s @ %s", ho.ModelName),
		CpuNum:          fmt.Sprintf("%d", cpuNum),
		TotalCpu:        ho.TotalNumberOfCores,
		UpSince:         getUpSince(),
		AgentVersion:    context.Version(),
		AgentName:       "Infrastructure",
		AgentMode:       context.Config().RunMode,
		OperatingSystem: "freeBSD",
		ProductUuid:     ho.HardwareUUID,
	}

	err = hip.setCloudRegion(data)
	if err != nil {
		hlog.WithError(err).WithField("cloudType", hip.cloudHarvester.GetCloudType()).Debug(
			"cloud region couldn't be set")
	}

	err = hip.setCloudMetadata(data)
	if err != nil {
		hlog.WithError(err).WithField("cloudType", hip.cloudHarvester.GetCloudType()).Debug(
			"cloud metadata couldn't be set")
	}

	helpers.LogStructureDetails(hlog, data, "HostInfoData", "raw", nil)

	return data
}

func (hip *HostinfoPlugin) getHostType(ho hardwareOverview) string {
	hostType := "unknown"

	if hip.Context.Config().DisableCloudMetadata ||
		hip.cloudHarvester.GetCloudType() == cloud.TypeNoCloud ||
		hip.cloudHarvester.GetCloudType() == cloud.TypeInProgress {

		hostType = fmt.Sprintf("%s %s", ho.ModelName)
	}

	if response, err := hip.cloudHarvester.GetHostType(); err != nil {
		hlog.WithError(err).WithField("cloudType", hip.cloudHarvester.GetCloudType()).Debug(
			"error getting host type from cloud metadata")
	} else {
		hostType = response
	}

	return hostType
}

func (hip *HostinfoPlugin) getKernelRelease() (string, error) {
	info, err := host.Info()
	if err != nil {
		hlog.WithError(err).Warn("unable to read host info for kernel release")
		return "", err
	}
	return info.KernelVersion, nil
}

func (hip *HostinfoPlugin) getOSDistribution() string {
	info, err := host.Info()
	if err != nil {
		return ""
	}
	return info.PlatformFamily
}

func getUpSince() string {
	info, err := host.Info()
	if err != nil {
		hlog.WithError(err).Warn("unable to read host info for uptime")
		return ""
	}
	return time.Now().Add(time.Second * -time.Duration(info.Uptime)).Format("2006-01-02 15:04:05")
}

func (hip *HostinfoPlugin) setCloudRegion(data *HostInfoData) (err error) {
	if hip.Context.Config().DisableCloudMetadata ||
		hip.cloudHarvester.GetCloudType() == cloud.TypeNoCloud {
		return
	}

	region, err := hip.cloudHarvester.GetRegion()
	if err != nil {
		return fmt.Errorf("couldn't retrieve cloud region: %v", err)
	}

	switch hip.cloudHarvester.GetCloudType() {
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

// Only for AWS cloud instances
func (self *HostinfoPlugin) setCloudMetadata(data *HostInfoData) (err error) {
	if self.Context.Config().DisableCloudMetadata ||
		self.cloudHarvester.GetCloudType() == cloud.TypeNoCloud {
		return
	}

	if self.cloudHarvester.GetCloudType() == cloud.TypeAWS {
		imageID, err := self.cloudHarvester.GetInstanceImageID()
		if err != nil {
			return fmt.Errorf("couldn't retrieve cloud image ID: %v", err)
		}
		data.AWSImageID = imageID
		awsAccountID, err := self.cloudHarvester.GetAccountID()
		if err != nil {
			return fmt.Errorf("couldn't retrieve cloud account ID: %v", err)
		}
		data.AWSAccountID = awsAccountID
		availabilityZone, err := self.cloudHarvester.GetZone()
		if err != nil {
			return fmt.Errorf("couldn't retrieve cloud availability zone: %v", err)
		}
		data.AWSAvailabilityZone = availabilityZone
	}
	return
}

// hardwareOverview structure keeps the values extracted from osX system_profiler.
type hardwareOverview struct {
	ModelName          string
	TotalNumberOfCores string
	NumberOfProcessors int
	HardwareUUID       string
}

func (hip *HostinfoPlugin) getHardwareOverview() (hardwareOverview, error) {
	result := hardwareOverview{}

	out, err := hip.readDataFromCmd("sysctl", "hw.model", "hw.machine", "hw.ncpu")
	if err != nil {
		return result, err
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "hw.model: ") {
			result.ModelName = strings.TrimPrefix(line, "hw.model: ")
		}

		if strings.HasPrefix(line, "hw.ncpu: ") {
			result.TotalNumberOfCores = strings.TrimPrefix(line, "hw.ncpu: ")
		}

	}

	cpuInfo, err := cpu.Info()
	if err != nil {
		return result, err
	}

	result.HardwareUUID = cpuInfo[0].PhysicalID
	result.NumberOfProcessors = 1

	return result, nil
}