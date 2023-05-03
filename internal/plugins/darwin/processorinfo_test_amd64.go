// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package darwin

import (
	"errors"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"testing"

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
			name:        "Intel arch output",
			unameOutput: "Darwin Kernel Version 20.3.0: Thu Jan 21 00:07:06 PST 2021; root:xnu-7195.81.3~1/RELEASE_X86_64`",
			systemProfilerOutput: `Hardware:

    Hardware Overview:

      Model Name: MacBook Pro
      Model Identifier: MacBookPro15,1
      Processor Name: 6-Core Intel Core i7
      Processor Speed: 2.6 GHz
      Number of Processors: 1
      Total Number of Cores: 6
      L2 Cache (per Core): 256 KB
      L3 Cache: 9 MB
      Hyper-Threading Technology: Enabled
      Memory: 16 GB
      System Firmware Version: 1554.80.3.0.0 (iBridge: 18.16.14347.0.0,0)
      Serial Number (system): abcd
      Hardware UUID: C3805006-DFCF-11EB-BA80-0242AC130004
`,
			expectedData: HostInfoDarwin{
				HostInfoData: common.HostInfoData{
					System:          "system",
					HostType:        "MacBook Pro MacBookPro15,1",
					CpuName:         "6-Core Intel Core i7 @ 2.6 GHz",
					CpuNum:          "6",
					TotalCpu:        "6",
					Ram:             "16777216 kB",
					UpSince:         "2021-07-01 09:59:30",
					AgentVersion:    "mock",
					AgentName:       "Infrastructure",
					OperatingSystem: "macOS",
				},
				Distro:        "macOS 11.2.3",
				KernelVersion: "20.3.0",
				AgentMode:     "root",
				ProductUuid:   "C3805006-DFCF-11EB-BA80-0242AC130004",
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
