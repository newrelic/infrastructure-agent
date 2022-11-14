// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

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

func TestGetHostInfo(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		assertions func(data *HostInfoData)
		setMock    func(*fakeHarvester)
	}{
		{
			name: "no cloud",
			assertions: func(d *HostInfoData) {
				assert.Equal(t, "", d.RegionAWS)
				assert.Equal(t, "", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
				assert.Equal(t, "system", d.System)
				assert.Equal(t, "Infrastructure", d.AgentName)
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
				assert.Equal(t, "system", d.System)
				assert.Equal(t, "Infrastructure", d.AgentName)
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
				assert.Equal(t, "northeurope", d.RegionAzure)
				assert.Equal(t, "", d.RegionGCP)
				assert.Equal(t, "", d.RegionAlibaba)
				assert.Equal(t, "1", d.AzureAvailabilityZone)
				assert.Equal(t, "x123", d.AzureSubscriptionID)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetAccountID").Return("x123", nil)
				h.On("GetCloudType").Return(cloud.TypeAzure)
				h.On("GetRegion").Return("northeurope", nil)
				h.On("GetZone").Return("1", nil)
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
		},
		{
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
			t.Parallel()
			h := new(fakeHarvester)
			testCase.setMock(h)
			hostInfo := NewHostInfoCommon("test", true, h)
			data, err := hostInfo.GetHostInfo()
			assert.NoError(t, err)
			testCase.assertions(&data)
			h.AssertExpectations(t)
		})
	}
}

func TestGetCloudHostType(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		assertions func(string, error)
		setMock    func(*fakeHarvester)
	}{
		{
			name: "no cloud",
			assertions: func(tp string, err error) {
				assert.Equal(t, "unknown", tp)
				assert.ErrorIs(t, ErrNoCloudHostTypeNotAvailable, err)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeNoCloud)
			},
		},
		{
			name: "cloud aws",
			assertions: func(tp string, err error) {
				assert.Equal(t, "t2.small", tp)
				assert.NoError(t, err)
			}, setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAWS)
				h.On("GetHostType").Return("t2.small", nil)
			},
		},
		{
			name: "cloud azure",
			assertions: func(tp string, err error) {
				assert.Equal(t, "Standard_DS2", tp)
				assert.NoError(t, err)
			}, setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAzure)
				h.On("GetHostType").Return("Standard_DS2", nil)
			},
		},
		{
			name: "cloud error",
			assertions: func(tp string, err error) {
				assert.Equal(t, "unknown", tp)
				assert.Error(t, err)
			}, setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAzure)
				h.On("GetHostType").Return("", errors.New("endpoint not available"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			h := new(fakeHarvester)
			testCase.setMock(h)
			hostInfo := NewHostInfoCommon("test", true, h)
			testCase.assertions(hostInfo.GetCloudHostType())
			h.AssertExpectations(t)
		})
	}
}
