// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux && harvest
// +build linux,harvest

package harvest

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"

	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func cpuInfoContent(totalCPU int, cpuCores int, cpuName string) string {

	var cpuInfo = ""
	for i := 0; i < totalCPU; i++ {
		cpuInfo += fmt.Sprintf(
			"processor : %d\n cpu cores : %d\n model name : %s\n",
			i,
			cpuCores,
			cpuName)
	}
	return cpuInfo
}

func TestHostInfo(t *testing.T) {
	const (
		agentVersion    = "mock agent version"
		agentIdentifier = "mock agent id"
		productUUID     = "01860EEC-200C-4DCC-9F4C-15BF3E5EC0DC"
		sysVendor       = "mock sys vendor"
		productName     = "mock product name"
		distroName      = "mock distro name"
		kernelVersion   = "mock kernel version"
		cpuName         = "mock model name"
		ram             = "42 kb"
		cpuCores        = 3
		totalCPU        = 2
		bootId          = "mock boot id"
	)

	var memInfo = fmt.Sprintf("MemTotal: %s", ram)

	ctx := new(mocks.AgentContext)
	ctx.On("AddReconnecting", mock.Anything).Return(nil)
	ctx.On("Config").Return(&config.Config{
		DisableCloudMetadata: true,
		RunMode:              config.ModePrivileged, // Used only to read the productUUID
	})
	ctx.On("Version").Return(agentVersion)
	ctx.On("EntityKey").Return(agentIdentifier)
	// Cannot assert the plugin payload here because `UpSince` is gotten from
	// running the `uptime` command.
	ctx.On("SendData", mock.Anything).Return(nil)
	ctx.SendDataWg.Add(1)

	// Mock the files used by the plugin to retrieve the system information
	sysFiles := []testhelpers.MockFile{
		{"class/dmi/id", "product_uuid", productUUID},
		{"devices/virtual/dmi/id", "sys_vendor", sysVendor},
		{"devices/virtual/dmi/id", "product_name", productName},
	}

	defer os.Clearenv()

	sysDir, err := testhelpers.NewMockDir(sysFiles)
	if err != nil {
		t.Fatal(err)
	}
	defer sysDir.Clear()
	os.Setenv("HOST_SYS", sysDir.Path)

	etcFiles := []testhelpers.MockFile{
		{"", "os-release", fmt.Sprintf("NAME=\"%s\n\"", distroName)},
	}
	etcDir, err := testhelpers.NewMockDir(etcFiles)
	if err != nil {
		t.Fatal(err)
	}
	defer etcDir.Clear()
	os.Setenv("HOST_ETC", etcDir.Path)

	procFiles := []testhelpers.MockFile{
		{"", "cpuinfo", cpuInfoContent(totalCPU, cpuCores, cpuName)},
		{"sys/kernel", "osrelease", kernelVersion},
		{"", "meminfo", memInfo},
		{"/sys/kernel/random/", "boot_id", bootId},
	}
	procDir, err := testhelpers.NewMockDir(procFiles)
	if err != nil {
		t.Fatal(err)
	}
	defer procDir.Clear()
	os.Setenv("HOST_PROC", procDir.Path)

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	hostInfoPlugin := pluginsLinux.NewHostinfoPlugin(ctx, common.NewHostInfoCommon(ctx.Version(), !ctx.Config().DisableCloudMetadata, cloudDetector))
	hostInfoPlugin.Run()
	ctx.AssertExpectations(t)

	// Retrieve the PluginOutput from the mock
	var actual agent.PluginOutput
	for _, call := range ctx.Calls {
		if call.Method == "SendData" {
			actual = call.Arguments[0].(agent.PluginOutput)
			break
		}
	}

	actualUpSince := actual.Data[0].(*pluginsLinux.HostInfoLinux).UpSince

	// The last |^$|unknown prevents the test to fail in some old linux distros where `uptime -s` returns
	// error because the -s argument is not accepted.
	uptimeRegex := regexp.MustCompile("^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}$|^$|unknown")
	assert.Regexp(t, uptimeRegex, actualUpSince)

	expectedPluginOutput := agent.PluginOutput{
		Id: ids.PluginID{
			Category: "metadata",
			Term:     "system",
		},
		Entity: entity.NewFromNameWithoutID(agentIdentifier),
		Data: agent.PluginInventoryDataset{
			&pluginsLinux.HostInfoLinux{
				HostInfoData: common.HostInfoData{
					System:          "system",
					HostType:        fmt.Sprintf("%s %s", sysVendor, productName),
					CpuName:         cpuName,
					CpuNum:          strconv.Itoa(cpuCores),
					TotalCpu:        strconv.Itoa(totalCPU),
					Ram:             ram,
					AgentVersion:    agentVersion,
					AgentName:       "Infrastructure",
					OperatingSystem: "linux",
					UpSince:         actualUpSince,
				},
				Distro:        distroName,
				KernelVersion: kernelVersion,
				ProductUuid:   productUUID,
				BootId:        bootId,
				AgentMode:     "privileged",
			},
		},
	}

	assert.Equal(t, expectedPluginOutput, actual)
}
