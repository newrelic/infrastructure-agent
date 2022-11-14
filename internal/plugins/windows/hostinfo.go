// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows
// +build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"golang.org/x/sys/windows/registry"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

var hlog = log.WithComponent("HostInfoPlugin")

var (
	modKernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modKernel32.NewProc("GetPhysicallyInstalledSystemMemory")
)

type HostinfoPlugin struct {
	agent.PluginCommon
	common.HostInfo
}

type HostInfoWindows struct {
	WindowsPlatform     string `json:"windows_platform"`
	WindowsFamily       string `json:"windows_family"`
	WindowsVersion      string `json:"windows_version"`
	common.HostInfoData `mapstructure:",squash"`
}

type cpuInfo struct {
	name     string
	num      string
	totalCpu string
}

func (self *HostInfoWindows) SortKey() string {
	return self.System
}

func NewHostinfoPlugin(id ids.PluginID, ctx agent.AgentContext, hostInfo common.HostInfo) agent.Plugin {
	return &HostinfoPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		HostInfo:     hostInfo,
	}
}

func (self *HostinfoPlugin) Data() agent.PluginInventoryDataset {
	info := getHostInfo()
	return agent.PluginInventoryDataset{self.gatherHostinfo(self.Context, info)}
}

func (self *HostinfoPlugin) Run() {
	self.Context.AddReconnecting(self)
	data := self.Data()
	self.EmitInventory(data, entity.NewFromNameWithoutID(self.Context.EntityKey()))
}

func getHostInfo() *host.InfoStat {
	info, err := host.Info()
	if err != nil {
		info = &host.InfoStat{
			OS: runtime.GOOS,
		}
	}
	return info
}

func (self *HostinfoPlugin) gatherHostinfo(context agent.AgentContext, info *host.InfoStat) *HostInfoWindows {
	commonHostInfo, err := self.GetHostInfo()
	if err != nil {
		hlog.WithError(err).Error("error fetching host data information")
	}

	cpuInfo := getCpuInfo()
	data := &HostInfoWindows{
		HostInfoData:    commonHostInfo,
		WindowsPlatform: info.Platform,
		WindowsFamily:   info.PlatformFamily,
		WindowsVersion:  info.PlatformVersion,
	}

	// set specific OS fields
	if data.HostType, err = self.GetCloudHostType(); err != nil {
		if errors.Is(err, common.ErrNoCloudHostTypeNotAvailable) {
			data.HostType = self.getHostType()
		} else {
			hlog.WithError(err).Debug("error getting host type from cloud metadata")
		}
	}

	data.CpuName = cpuInfo.name
	data.CpuNum = cpuInfo.num
	data.TotalCpu = cpuInfo.totalCpu
	data.Ram = getRam()
	data.UpSince = time.Unix(int64(info.BootTime), 0).Format("2006-01-02 15:04:05")
	data.OperatingSystem = info.OS

	helpers.LogStructureDetails(hlog, data, "HostInfoData", "raw", nil)

	return data
}

func getCpuInfo() *cpuInfo {
	// We set up a context with a large deadline because if the context
	// does not provide a deadline, Gopsutil sets a default 3-seconds deadline
	// that could be too low for some systems.
	var maxTimeout = 100 * 365 * 24 * time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 100*maxTimeout)

	info, err := cpu.InfoWithContext(ctx)

	if err != nil {
		defer cancel()
		hlog.WithError(err).Debug("Error getting cpu info.")
		return &cpuInfo{}
	}
	localCpu := int64(info[0].Cores)
	totalCpu := len(info)

	data := &cpuInfo{
		name:     info[0].ModelName,
		num:      strconv.FormatInt(localCpu, 10),
		totalCpu: strconv.Itoa(totalCpu),
	}
	return data
}

func getRam() string {
	mem, err := mem.VirtualMemory()
	if err != nil {
		hlog.WithError(err).Debug("Error getting memory info.")
		return "unknown"
	}
	totalMem := mem.Total
	return strconv.FormatUint(totalMem, 10)
}

func (self *HostinfoPlugin) getHostType() string {
	hostType := "unknown"

	if regKey, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\SystemInformation\`, registry.QUERY_VALUE); err == nil {
		Manufacturer, _, _ := regKey.GetStringValue("SystemManufacturer")
		ProductName, _, _ := regKey.GetStringValue("SystemProductName")

		if Manufacturer != "" && ProductName != "" {
			hostType = strings.Trim(fmt.Sprintf("%s %s", Manufacturer, ProductName), " ")
		}
	} else {
		log.WithError(err).Debug("Error getting host type from Windows Registry.")
	}

	return hostType
}
