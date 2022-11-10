// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package darwin

import (
	"errors"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"gotest.tools/assert"
)

var ErrUnknownCommand = errors.New("unknown command")

func TestData(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                 string
		unameOutput          string
		systemProfilerOutput string
		expectedData         HostInfoDarwin
	}{
		{
			name:        "Arm arch output",
			unameOutput: "Darwin Kernel Version 21.6.0: Wed Aug 10 14:28:23 PDT 2022; root:xnu-8020.141.5~2/RELEASE_ARM64_T6000",
			systemProfilerOutput: `Hardware:

    Hardware overview:

      Model Name: MacBook Pro
      Model Identifier: MacBookPro18,4
      Chip: Apple M1 Max
      Total Number of Cores: 10 (8 performance and 2 efficiency)
      Memory: 64 GB
      System Firmware Version: 7411.111.1
      OS Loader Version: 7411.111.1
      Serial Number (system): H9AAAAA9WK
      Hardware UUID: E62094F3-9A33-5555-5555-F4C3A1E16AC5
      Provisioning UDID: 00006000-000000000C1A801E
      Activation Lock Status: Disabled
`,
			expectedData: HostInfoDarwin{
				HostInfoData: common.HostInfoData{
					System:          "system",
					HostType:        "MacBook Pro MacBookPro18,4",
					CpuName:         "Apple M1 Max",
					CpuNum:          "10",
					TotalCpu:        "10",
					Ram:             "67108864 kB",
					UpSince:         "2021-07-01 09:59:30",
					AgentVersion:    "mock",
					AgentName:       "Infrastructure",
					OperatingSystem: "macOS",
				},
				Distro:        "macOS 11.2.3",
				KernelVersion: "21.6.0",
				AgentMode:     "root",
				ProductUuid:   "E62094F3-9A33-5555-5555-F4C3A1E16AC5",
			},
		},
	}

	for _, tt := range testCases {
		tt := tt // nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hip := HostinfoPlugin{
				PluginCommon: agent.PluginCommon{
					ID:      ids.HostInfo,
					Context: testing2.NewMockAgent(),
				},
				readDataFromCmd: func(cmd string, args ...string) (string, error) {
					if cmd == "system_profiler" {
						return tt.systemProfilerOutput, nil
					} else if cmd == "uname" {
						return tt.unameOutput, nil
					}

					return ``, ErrUnknownCommand
				},
				HostInfo: common.NewHostInfoCommon("test", true, cloud.NewDetector(true, 0, 0, 0, true)),
			}
			hip.Context.Config().DisableCloudMetadata = true
			hip.Context.Config().RunMode = "root"
			// Some values (distro, upSince) are being read from the host, so commented until fix those (out of scope in this task)
			// assert.Equal(t, expected, hip.Data()[0])
			data, ok := hip.Data()[0].(*HostInfoDarwin)
			assert.Equal(t, true, ok)

			assert.Equal(t, tt.expectedData.System, data.System)
			// assert.Equal(t, expected.Distro, data.Distro)
			assert.Equal(t, tt.expectedData.KernelVersion, data.KernelVersion)
			assert.Equal(t, tt.expectedData.HostType, data.HostType)
			assert.Equal(t, tt.expectedData.CpuName, data.CpuName)
			assert.Equal(t, tt.expectedData.CpuNum, data.CpuNum)
			assert.Equal(t, tt.expectedData.TotalCpu, data.TotalCpu)
			assert.Equal(t, tt.expectedData.Ram, data.Ram)
			// assert.Equal(t, expected.UpSince, data.UpSince)
			assert.Equal(t, tt.expectedData.AgentVersion, data.AgentVersion)
			assert.Equal(t, tt.expectedData.AgentName, data.AgentName)
			assert.Equal(t, tt.expectedData.AgentMode, data.AgentMode)
			assert.Equal(t, tt.expectedData.OperatingSystem, data.OperatingSystem)
			assert.Equal(t, tt.expectedData.ProductUuid, data.ProductUuid)
		})
	}
}
