// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package darwin

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetKernelRelease_Success(t *testing.T) {
	hip := HostinfoPlugin{
		readDataFromCmd: func(cmd string, args ...string) (string, error) {
			return "Darwin Kernel Version 20.3.0: Thu Jan 21 00:07:06 PST 2021; root:xnu-7195.81.3~1/RELEASE_X86_64", nil
		},
	}
	actual, err := hip.getKernelRelease()

	assert.NoError(t, err)
	assert.Equal(t, "20.3.0", actual)
}

func TestGetKernelRelease_Error(t *testing.T) {
	hip := HostinfoPlugin{
		readDataFromCmd: func(cmd string, args ...string) (string, error) {
			return "", errors.New("error")
		},
	}
	actual, err := hip.getKernelRelease()

	assert.Error(t, err)
	assert.Equal(t, "", actual)
}

func TestGetKernelRelease_BadFormat(t *testing.T) {
	hip := HostinfoPlugin{
		readDataFromCmd: func(cmd string, args ...string) (string, error) {
			return "root:xnu-7195.81.3~1/RELEASE_X86_64", nil
		},
	}
	actual, err := hip.getKernelRelease()

	assert.Error(t, err)
	assert.Equal(t, "", actual)
}

func TestMemoryToKb(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "uppercase_correct_format",
			input:    "16 GB",
			expected: "16777216 kB",
			hasError: false,
		},
		{
			name:     "lowercase_correct_format",
			input:    "16 gB",
			expected: "16777216 kB",
			hasError: false,
		},
		{
			name:     "wrong_format",
			input:    "1aaas",
			expected: "",
			hasError: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual, err := memoryToKb(test.input)
			if test.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestValidateHardwareUUID(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "uppercase_correct_format",
			input:    "C3805006-DFCF-11EB-BA80-0242AC130004",
			expected: true,
		},
		{
			name:     "lowercase_wrong_format",
			input:    "c3805006-dfcf-11eb-ba80-0242ac130004",
			expected: false,
		},
		{
			name:     "wrong_format",
			input:    "err",
			expected: false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, validateHardwareUUID(test.input))
		})
	}
}

func TestSplitRightSubstring(t *testing.T) {
	var testCases = []struct {
		name      string
		output    string
		substring string
		separator string
		expected  string
	}{
		{name: "Empty input",
			output:    "",
			substring: "",
			separator: "",
			expected:  "",
		},
		{name: "Empty substring",
			output:    "Hello bye yes",
			substring: "",
			separator: "$",
			expected:  "",
		},
		{name: "Empty separator",
			output:    "Hello bye yes",
			substring: "bye",
			separator: "",
			expected:  "",
		},
		{name: "Word separator",
			output:    "Hello bye yes",
			substring: "Hello ",
			separator: " yes",
			expected:  "bye",
		},
		{name: "Dot separator",
			output:    "Fosdem: A lot of questions.",
			substring: "Fosdem: ",
			separator: ".",
			expected:  "A lot of questions",
		},
		{name: "Newline separator",
			output: `Fosdem: A lot of questions
`,
			substring: "Fosdem: ",
			separator: "\n",
			expected:  "A lot of questions",
		},
		{name: "Substring high slice bound",
			output:    "Fosdem: A lot of questions",
			substring: "Fosdem: A lot of questions",
			separator: "",
			expected:  "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(
				t,
				splitRightSubstring(tt.output,
					tt.substring,
					tt.separator,
				),
				tt.expected)
		})
	}
}

func TestData(t *testing.T) {
	var testCases = []struct {
		name                 string
		unameOutput          string
		systemProfilerOutput string
		expectedData         HostInfoData
	}{
		{name: "Intel arch output",
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
			expectedData: HostInfoData{
				System:          "system",
				Distro:          "macOS 11.2.3",
				KernelVersion:   "20.3.0",
				HostType:        "MacBook Pro MacBookPro15,1",
				CpuName:         "6-Core Intel Core i7 @ 2.6 GHz",
				CpuNum:          "6",
				TotalCpu:        "6",
				Ram:             "16777216 kB",
				UpSince:         "2021-07-01 09:59:30",
				AgentVersion:    "mock",
				AgentName:       "Infrastructure",
				AgentMode:       "root",
				OperatingSystem: "macOS",
				ProductUuid:     "C3805006-DFCF-11EB-BA80-0242AC130004",
			},
		},
		{name: "Arm arch output",
			unameOutput: "Darwin Kernel Version 21.6.0: Wed Aug 10 14:28:23 PDT 2022; root:xnu-8020.141.5~2/RELEASE_ARM64_T6000",
			systemProfilerOutput: `Hardware:

    Hardware overview:

      Model Name: MacBook Pro
      Model Identifier: MacBookPro18,4
      Chip: Apple M1 Max
      Total Number of Cores: 10 (8 performance and 2 efficiency)
      Memory: 64 GB
      System Firmware Version: 7459.141.1
      OS Loader Version: 7459.141.1
      Serial Number (system): H9AAAAA9WK
      Hardware UUID: E62094F3-9A33-5555-5555-F4C3A1E16AC5
      Provisioning UDID: 00006000-000000000C1A801E
      Activation Lock Status: Disabled
`,
			expectedData: HostInfoData{
				System:          "system",
				Distro:          "macOS 11.2.3",
				KernelVersion:   "21.6.0",
				HostType:        "MacBook Pro MacBookPro18,4",
				CpuName:         "Apple M1 Max",
				CpuNum:          "10",
				TotalCpu:        "10",
				Ram:             "67108864 kB",
				UpSince:         "2021-07-01 09:59:30",
				AgentVersion:    "mock",
				AgentName:       "Infrastructure",
				AgentMode:       "root",
				OperatingSystem: "macOS",
				ProductUuid:     "E62094F3-9A33-5555-5555-F4C3A1E16AC5",
			},
		},
	}

	for _, tt := range testCases {
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

					return ``, errors.New("unknown command")
				},

				cloudHarvester: cloud.NewDetector(true, 0, 0, 0, true),
			}
			hip.Context.Config().DisableCloudMetadata = true
			hip.Context.Config().RunMode = "root"
			//Some values (distro, upSince) are being read from the host, so commented until fix those (out of scope in this task)
			//assert.Equal(t, expected, hip.Data()[0])
			data, ok := hip.Data()[0].(*HostInfoData)
			assert.Equal(t, true, ok)

			assert.Equal(t, tt.expectedData.System, data.System)
			//assert.Equal(t, expected.Distro, data.Distro)
			assert.Equal(t, tt.expectedData.KernelVersion, data.KernelVersion)
			assert.Equal(t, tt.expectedData.HostType, data.HostType)
			assert.Equal(t, tt.expectedData.CpuName, data.CpuName)
			assert.Equal(t, tt.expectedData.CpuNum, data.CpuNum)
			assert.Equal(t, tt.expectedData.TotalCpu, data.TotalCpu)
			assert.Equal(t, tt.expectedData.Ram, data.Ram)
			//assert.Equal(t, expected.UpSince, data.UpSince)
			assert.Equal(t, tt.expectedData.AgentVersion, data.AgentVersion)
			assert.Equal(t, tt.expectedData.AgentName, data.AgentName)
			assert.Equal(t, tt.expectedData.AgentMode, data.AgentMode)
			assert.Equal(t, tt.expectedData.OperatingSystem, data.OperatingSystem)
			assert.Equal(t, tt.expectedData.ProductUuid, data.ProductUuid)
		})
	}

}

type fakeHarvester struct {
	mock.Mock
}

// GetInstanceID will return the id of the cloud instance.
func (f *fakeHarvester) GetInstanceID() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetHostType will return the cloud instance type.
func (f *fakeHarvester) GetHostType() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetCloudType will return the cloud type on which the instance is running.
func (f *fakeHarvester) GetCloudType() cloud.Type {
	args := f.Called()
	return args.Get(0).(cloud.Type)
}

// Returns a string key which will be used as a HostSource (see host_aliases plugin).
func (f *fakeHarvester) GetCloudSource() string {
	args := f.Called()
	return args.String(0)
}

// GetRegion returns the cloud region
func (f *fakeHarvester) GetRegion() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetZone returns the cloud zone (availability zone)
func (f *fakeHarvester) GetZone() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetAccount returns the cloud account ID
func (f *fakeHarvester) GetAccountID() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetImageID returns the cloud instance ID
func (f *fakeHarvester) GetInstanceImageID() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

// GetHarvester returns instance of the Harvester detected (or instance of themselves)
func (f *fakeHarvester) GetHarvester() (cloud.Harvester, error) {
	return f, nil
}

func TestHostinfoPluginSetCloudRegion(t *testing.T) {
	testCases := []struct {
		name       string
		assertions func(*HostInfoData)
		setMock    func(*fakeHarvester)
	}{
		{
			name: "no cloud",
			assertions: func(d *HostInfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeNoCloud)
			},
		},
		{
			name: "cloud aws",
			assertions: func(d *HostInfoData) {
				assert.Equal(t, "us-east-1", d.RegionAWS)
				assert.Equal(t, "us-east-1a", d.AWSAvailabilityZone)
				assert.Equal(t, "ami-12345", d.AWSImageID)
				assert.Equal(t, "x123", d.AWSAccountID)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAWS)
				h.On("GetRegion").Return("us-east-1", nil)
				h.On("GetZone").Return("us-east-1a", nil)
				h.On("GetInstanceImageID").Return("ami-12345", nil)
				h.On("GetAccountID").Return("x123", nil)
			},
		},
		{
			name: "cloud azure",
			assertions: func(d *HostInfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "us-east-1", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAzure)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		},
		{
			name: "cloud gcp",
			assertions: func(d *HostInfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "us-east-1", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeGCP)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		}, {
			name: "cloud alibaba",
			assertions: func(d *HostInfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "us-east-1", d.RegionAlibaba)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAlibaba)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			h := new(fakeHarvester)
			testCase.setMock(h)
			data := &HostInfoData{}
			p := &HostinfoPlugin{
				PluginCommon: agent.PluginCommon{
					ID:      ids.HostInfo,
					Context: testing2.NewMockAgent(),
				},
				cloudHarvester: h,
			}
			_ = p.setCloudRegion(data)
			_ = p.setCloudMetadata(data)
			testCase.assertions(data)
			h.AssertExpectations(t)
		})
	}
}
