// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package darwin

import (
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
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

type HostInfoMock struct {
	getHostInfo      func() (common.HostInfoData, error)
	getCloudHostType func() (string, error)
}

func (h *HostInfoMock) GetHostInfo() (common.HostInfoData, error) {
	return h.getHostInfo()

}

func (h *HostInfoMock) GetCloudHostType() (string, error) {
	return h.getCloudHostType()
}

func TestGatherHostInfoCloud(t *testing.T) {
	var testCases = []struct {
		mockedGetHostInfo      func() (common.HostInfoData, error)
		mockedGetCloudHostType func() (string, error)
		assertions             func(data *HostInfoDarwin)
	}{
		{
			mockedGetHostInfo: func() (common.HostInfoData, error) {
				data := common.HostInfoData{}
				data.RegionAWS = "eu-west-us"
				data.AWSImageID = "ubuntu"
				data.AWSAccountID = "000"
				data.AWSAvailabilityZone = "3"
				return data, nil
			},
			mockedGetCloudHostType: func() (string, error) {
				return "", nil
			},
			assertions: func(data *HostInfoDarwin) {
				assert.Equal(t, "eu-west-us", data.RegionAWS)
				assert.Equal(t, "ubuntu", data.AWSImageID)
				assert.Equal(t, "000", data.AWSAccountID)
				assert.Equal(t, "3", data.AWSAvailabilityZone)
			},
		},
		{
			mockedGetHostInfo: func() (common.HostInfoData, error) {
				data := common.HostInfoData{}
				data.RegionAzure = "eu-west-us"
				data.AzureSubscriptionID = "1234"
				data.AzureAvailabilityZone = "1"
				return data, nil
			},
			mockedGetCloudHostType: func() (string, error) {
				return "", nil
			},
			assertions: func(data *HostInfoDarwin) {
				assert.Equal(t, "eu-west-us", data.RegionAzure)
				assert.Equal(t, "1", data.AzureAvailabilityZone)
				assert.Equal(t, "1234", data.AzureSubscriptionID)
			},
		},
	}

	hip := HostinfoPlugin{
		readDataFromCmd: func(cmd string, args ...string) (string, error) {
			return "", errors.New("error")
		},
		HostInfo: &HostInfoMock{},
	}

	for _, tt := range testCases {
		t.Run("", func(t *testing.T) {
			hip.HostInfo = &HostInfoMock{
				getHostInfo:      tt.mockedGetHostInfo,
				getCloudHostType: tt.mockedGetCloudHostType,
			}
			actual := hip.gatherHostinfo(testing2.NewMockAgent())
			tt.assertions(actual)
		})
	}

}
