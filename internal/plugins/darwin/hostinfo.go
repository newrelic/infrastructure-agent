// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package darwin

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/os/distro"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/sirupsen/logrus"
)

var (
	hlog               = log.WithComponent("HostInfoPlugin")
	kernelVersionRegex = regexp.MustCompile("Version (.*?):")
)

// HostinfoPlugin returns metadata of the host.
type HostinfoPlugin struct {
	agent.PluginCommon
	common.HostInfo

	readDataFromCmd func(cmd string, args ...string) (string, error)
}

// HostInfoDarwin is the structure returned to the backend by HostinfoPlugin.
type HostInfoDarwin struct {
	Distro        string `json:"distro"`
	KernelVersion string `json:"kernel_version"`

	AgentMode           string `json:"agent_mode"`
	ProductUuid         string `json:"product_uuid"`
	common.HostInfoData `mapstructure:",squash"`
}

func (hip *HostInfoDarwin) SortKey() string {
	return hip.System
}

func NewHostinfoPlugin(ctx agent.AgentContext, hostInfo common.HostInfo) agent.Plugin {
	return &HostinfoPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.HostInfo,
			Context: ctx,
		},
		HostInfo: hostInfo,
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

func (hip *HostinfoPlugin) gatherHostinfo(context agent.AgentContext) *HostInfoDarwin {
	ho, err := hip.getHardwareOverview()
	if err != nil {
		hlog.WithError(err).Error("error reading hardware overview")
	}

	kernelVersion, err := hip.getKernelRelease()
	if err != nil {
		hlog.WithError(err).Error("error reading kernel release")
	}

	numberOfProcessors, err := strconv.Atoi(ho.NumberOfProcessors)
	if err != nil {
		hlog.WithError(err).Error("error converting number of processors to int")
	}
	totalNumberOfCores, err := strconv.Atoi(ho.TotalNumberOfCores)
	if err != nil {
		hlog.WithError(err).Error("error converting total number of cores to int")
	}

	cpuNum := 0
	if numberOfProcessors > 0 {
		cpuNum = totalNumberOfCores / numberOfProcessors
	}

	cpuName := ho.ProcessorName
	// arm architectures do not provide processor speed
	if ho.ProcessorSpeed != "" {
		cpuName = fmt.Sprintf("%s @ %s", ho.ProcessorName, ho.ProcessorSpeed)
	}

	commonHostInfo, err := hip.GetHostInfo()
	if err != nil {
		hlog.WithError(err).Error("error fetching host data information")
	}

	data := &HostInfoDarwin{
		HostInfoData:  commonHostInfo,
		Distro:        distro.GetDistro(),
		KernelVersion: kernelVersion,
		AgentMode:     context.Config().RunMode,
		ProductUuid:   ho.HardwareUUID,
	}

	// set specific OS fields
	if data.HostType, err = hip.GetCloudHostType(); err != nil {
		if errors.Is(err, common.ErrNoCloudHostTypeNotAvailable) {
			data.HostType = hip.getHostType(ho)
		} else {
			hlog.WithError(err).Debug("error getting host type from cloud metadata")
		}
	}

	data.CpuName = cpuName
	data.CpuNum = fmt.Sprintf("%d", cpuNum)
	data.TotalCpu = ho.TotalNumberOfCores
	data.Ram = ho.Memory
	data.UpSince = getUpSince()
	data.OperatingSystem = "macOS"

	helpers.LogStructureDetails(hlog, data, "HostInfoDarwin", "raw", nil)

	return data
}

func (hip *HostinfoPlugin) getHostType(ho hardwareOverview) string {
	return fmt.Sprintf("%s %s", ho.ModelName, ho.ModelIdentifier)
}

func (hip *HostinfoPlugin) getKernelRelease() (string, error) {
	out, err := hip.readDataFromCmd("uname", "-v")
	if err != nil {
		return "", err
	}

	if kernelVersion := kernelVersionRegex.FindStringSubmatch(out); len(kernelVersion) > 1 {
		return kernelVersion[1], nil
	}
	return "", fmt.Errorf("failed to detect kernel version in: '%s'", out)
}

func getUpSince() string {
	info, err := host.Info()
	if err != nil {
		hlog.WithError(err).Warn("unable to read host info for uptime")
		return ""
	}
	return time.Now().Add(time.Second * -time.Duration(info.Uptime)).Format("2006-01-02 15:04:05")
}

// hardwareOverview structure keeps the values extracted from osX system_profiler.
type hardwareOverview struct {
	ModelName       string
	ModelIdentifier string
	Memory          string
	HardwareUUID    string
	processorInfo
}

// processorInfo structure keeps the values extracted from osX system_profiler
// for the processor information.
type processorInfo struct {
	ProcessorName      string
	ProcessorSpeed     string
	NumberOfProcessors string
	TotalNumberOfCores string
}

func (hip *HostinfoPlugin) getHardwareOverview() (hardwareOverview, error) {
	out, err := hip.readDataFromCmd("system_profiler", "SPHardwareDataType")
	if err != nil {
		return hardwareOverview{}, err
	}

	cpuInfo := getProcessorData(out)

	memory, err := memoryToKb(helpers.SplitRightSubstring(out, "Memory: ", "\n"))
	if err != nil {
		hlog.WithFields(logrus.Fields{
			"line": helpers.SplitRightSubstring(out, "Memory: ", "\n"),
		}).Debug("Unexpected format for 'Memory' field.")
	}

	uuid := helpers.SplitRightSubstring(out, "Hardware UUID: ", "\n")
	if !validateHardwareUUID(uuid) {
		hlog.WithError(err).Debug("Error detecting hardware UUID")
		uuid = "unknown"
	}

	return hardwareOverview{
		helpers.SplitRightSubstring(out, "Model Name: ", "\n"),
		helpers.SplitRightSubstring(out, "Model Identifier: ", "\n"),
		memory,
		uuid,
		cpuInfo,
	}, nil
}

func validateHardwareUUID(uuid string) bool {
	matched, err := regexp.MatchString("^[0-9A-F]{8}(-[0-9A-F]{4}){3}-[0-9A-F]{12}$", uuid)
	if err != nil {
		return false
	}
	return matched
}

func memoryToKb(line string) (string, error) {
	fields := strings.Fields(strings.TrimPrefix(line, "Memory: "))
	if len(fields) != 2 {
		return "", fmt.Errorf("expected 2 fields but got: %d", len(fields))
	}

	if strings.ToLower(fields[1]) != "gb" {
		return "", fmt.Errorf("unexpected unit '%s'", fields[1])
	}
	mem, err := strconv.Atoi(fields[0])
	if err != nil {
		return "", fmt.Errorf("failed to parse memory field: %s", err)
	}
	return fmt.Sprintf("%d kB", mem*1024*1024), nil
}
