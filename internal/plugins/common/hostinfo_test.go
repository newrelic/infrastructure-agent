// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"runtime"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// errInstancePrincipalUnavailable is a stand-in for the real OCI SDK error returned when
// instance principal authentication is unavailable, used to test Phase 2 graceful degradation.
var errInstancePrincipalUnavailable = errors.New("instance principal unavailable")

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

// GetInstanceDisplayName returns the cloud instance display name
func (f *fakeHarvester) GetInstanceDisplayName() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetVMSize returns the cloud instance VM size.
func (f *fakeHarvester) GetVMSize() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetFaultDomain returns the cloud instance fault domain.
func (f *fakeHarvester) GetFaultDomain() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetHostname returns the cloud instance hostname.
func (f *fakeHarvester) GetHostname() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetFreeformTags returns the cloud instance freeform tags.
func (f *fakeHarvester) GetFreeformTags() (map[string]string, error) {
	args := f.Called()

	tags, _ := args.Get(0).(map[string]string)

	return tags, args.Error(1) //nolint:wrapcheck
}

// GetPrivateIP returns the cloud instance private IP.
func (f *fakeHarvester) GetPrivateIP() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetVCNID returns the cloud instance VCN ID.
func (f *fakeHarvester) GetVCNID() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetSubnetID returns the cloud instance subnet ID.
func (f *fakeHarvester) GetSubnetID() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetLifecycleState returns the cloud instance lifecycle state.
func (f *fakeHarvester) GetLifecycleState() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetVirtualizationType returns the cloud instance virtualization type.
func (f *fakeHarvester) GetVirtualizationType() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetDedicatedVMHostID returns the cloud instance dedicated VM host ID.
func (f *fakeHarvester) GetDedicatedVMHostID() (string, error) {
	args := f.Called()

	return args.String(0), args.Error(1)
}

// GetHarvester returns instance of the Harvester detected (or instance of themselves)
func (f *fakeHarvester) GetHarvester() (cloud.Harvester, error) {
	return f, nil
}

// setMockOCIPhase1 sets up the shared Phase 1 (IMDS) mock expectations for OCI test cases,
// leaving Phase 2 (OCI API) expectations for the caller to set based on the scenario.
func setMockOCIPhase1(harvester *fakeHarvester) {
	harvester.On("GetAccountID").Return("ocid1.compartment.oc1", nil)
	harvester.On("GetCloudType").Return(cloud.TypeOCI)
	harvester.On("GetRegion").Return("us-ashburn-1", nil)
	harvester.On("GetZone").Return("jyDh:US-ASHBURN-AD-1", nil)
	harvester.On("GetInstanceImageID").Return("ocid1.image.oc1", nil)
	harvester.On("GetInstanceDisplayName").Return("ubunut-instance-20250722-1328", nil)
	harvester.On("GetVMSize").Return("VM.Optimized3.Flex", nil)
	harvester.On("GetFaultDomain").Return("FAULT-DOMAIN-1", nil)
	harvester.On("GetHostname").Return("ubunut-instance-20250722-1328", nil)
	harvester.On("GetPrivateIP").Return("10.0.0.5", nil)
	harvester.On("GetFreeformTags").Return(map[string]string{"env": "prod"}, nil)
	harvester.On("GetInstanceID").Return("ocid1.instance.oc1", nil)
}

func TestGetHostInfo(t *testing.T) {
	t.Parallel()

	agentTestVersion := "test"

	testCases := []struct {
		name       string
		assertions func(data *HostInfoData, err error)
		setMock    func(*fakeHarvester)
	}{
		{
			name: "no cloud",
			assertions: func(data *HostInfoData, err error) {
				assert.Equal(t, "", data.RegionAWS)
				assert.Equal(t, "", data.RegionAzure)
				assert.Equal(t, "", data.RegionGCP)
				assert.Equal(t, "", data.RegionAlibaba)
				assert.Equal(t, "", data.RegionOCI)
				assert.Equal(t, "system", data.System)
				assert.Equal(t, "Infrastructure", data.AgentName)
				assert.NoError(t, err)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeNoCloud)
			},
		},
		{
			name: "cloud aws",
			assertions: func(data *HostInfoData, err error) {
				assert.Equal(t, "us-east-1", data.RegionAWS)
				assert.Equal(t, "us-east-1a", data.AWSAvailabilityZone)
				assert.Equal(t, "ami-12345", data.AWSImageID)
				assert.Equal(t, "x123", data.AWSAccountID)
				assert.Equal(t, "", data.RegionAzure)
				assert.Equal(t, "", data.RegionGCP)
				assert.Equal(t, "", data.RegionAlibaba)
				assert.Equal(t, "", data.RegionOCI)
				assert.Equal(t, "system", data.System)
				assert.Equal(t, "Infrastructure", data.AgentName)
				assert.NoError(t, err)
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
			assertions: func(data *HostInfoData, err error) {
				assert.Equal(t, "", data.RegionAWS)
				assert.Equal(t, "northeurope", data.RegionAzure)
				assert.Equal(t, "", data.RegionGCP)
				assert.Equal(t, "", data.RegionAlibaba)
				assert.Equal(t, "", data.RegionOCI)
				assert.Equal(t, "1", data.AzureAvailabilityZone)
				assert.Equal(t, "x123", data.AzureSubscriptionID)
				assert.NoError(t, err)
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
			assertions: func(data *HostInfoData, err error) {
				assert.Equal(t, "", data.RegionAWS)
				assert.Equal(t, "", data.RegionAzure)
				assert.Equal(t, "", data.RegionOCI)
				assert.Equal(t, "us-east-1", data.RegionGCP)
				assert.Equal(t, "", data.RegionAlibaba)
				assert.NoError(t, err)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeGCP)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		},
		{
			name: "cloud alibaba",
			assertions: func(data *HostInfoData, err error) {
				assert.Equal(t, "", data.RegionAWS)
				assert.Equal(t, "", data.RegionAzure)
				assert.Equal(t, "", data.RegionGCP)
				assert.Equal(t, "", data.RegionOCI)
				assert.Equal(t, "us-east-1", data.RegionAlibaba)
				assert.NoError(t, err)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAlibaba)
				h.On("GetRegion").Return("us-east-1", nil)
			},
		},
		{
			name: "cloud oci",
			assertions: func(data *HostInfoData, err error) {
				assert.Equal(t, "", data.RegionAWS)
				assert.Equal(t, "", data.RegionAzure)
				assert.Equal(t, "", data.RegionGCP)
				assert.Equal(t, "", data.RegionAlibaba)
				assert.Equal(t, "us-ashburn-1", data.RegionOCI)
				assert.Equal(t, "ocid1.compartment.oc1", data.OCIAccountID)
				assert.Equal(t, "jyDh:US-ASHBURN-AD-1", data.OCIAvailabilityZone)
				assert.Equal(t, "ocid1.image.oc1", data.OCIImageID)
				assert.Equal(t, "ubunut-instance-20250722-1328", data.OCIDisplayName)
				assert.Equal(t, "VM.Optimized3.Flex", data.OCIVMSize)
				assert.Equal(t, "ocid1.instance.oc1", data.CloudResourceID)
				assert.Equal(t, "ocid1.instance.oc1", data.OCIInstanceID)
				assert.Equal(t, "FAULT-DOMAIN-1", data.OCIFaultDomain)
				assert.Equal(t, "ubunut-instance-20250722-1328", data.OCIPrivateDNSName)
				assert.Equal(t, "10.0.0.5", data.OCIPrivateIP)
				assert.Equal(t, runtime.GOARCH, data.HostArch)
				assert.Equal(t, map[string]string{"env": "prod"}, data.OCIFreeformTags)
				assert.Equal(t, "oci", data.CloudProvider)
				assert.Equal(t, "oci_compute", data.CloudPlatform)
				assert.Equal(t, "ocid1.vcn.oc1", data.OCIVCNID)
				assert.Equal(t, "ocid1.subnet.oc1", data.OCISubnetID)
				assert.Equal(t, "RUNNING", data.OCILifecycleState)
				assert.Equal(t, "NATIVE", data.OCIVirtualizationType)
				assert.Equal(t, "ocid1.dedicatedvmhost.oc1", data.OCIDedicatedVMHostID)
				assert.NoError(t, err)
			},
			setMock: func(harvester *fakeHarvester) {
				setMockOCIPhase1(harvester)
				harvester.On("GetVCNID").Return("ocid1.vcn.oc1", nil)
				harvester.On("GetSubnetID").Return("ocid1.subnet.oc1", nil)
				harvester.On("GetLifecycleState").Return("RUNNING", nil)
				harvester.On("GetVirtualizationType").Return("NATIVE", nil)
				harvester.On("GetDedicatedVMHostID").Return("ocid1.dedicatedvmhost.oc1", nil)
			},
		},
		{
			name: "cloud oci - phase 2 API unavailable",
			assertions: func(data *HostInfoData, err error) {
				// Phase 1 (IMDS) attributes still populate.
				assert.Equal(t, "us-ashburn-1", data.RegionOCI)
				assert.Equal(t, "ocid1.instance.oc1", data.CloudResourceID)
				// Phase 2 (OCI API) attributes are left empty, not an error - graceful degradation.
				assert.Empty(t, data.OCIVCNID)
				assert.Empty(t, data.OCISubnetID)
				assert.Empty(t, data.OCILifecycleState)
				assert.Empty(t, data.OCIVirtualizationType)
				assert.Empty(t, data.OCIDedicatedVMHostID)
				assert.NoError(t, err)
			},
			setMock: func(harvester *fakeHarvester) {
				setMockOCIPhase1(harvester)
				harvester.On("GetVCNID").Return("", errInstancePrincipalUnavailable)
				harvester.On("GetSubnetID").Return("", errInstancePrincipalUnavailable)
				harvester.On("GetLifecycleState").Return("", errInstancePrincipalUnavailable)
				harvester.On("GetVirtualizationType").Return("", errInstancePrincipalUnavailable)
				harvester.On("GetDedicatedVMHostID").Return("", errInstancePrincipalUnavailable)
			},
		},
		{
			name: "cloud error",
			assertions: func(data *HostInfoData, err error) {
				assert.Equal(t, "", data.RegionAWS)
				assert.Equal(t, "", data.RegionAzure)
				assert.Equal(t, "", data.RegionGCP)
				assert.Equal(t, "", data.RegionAlibaba)
				assert.Equal(t, "", data.RegionOCI)
				assert.Equal(t, "system", data.System)
				assert.Equal(t, "Infrastructure", data.AgentName)
				assert.Equal(t, agentTestVersion, data.AgentVersion)
				assert.Error(t, err)
			},
			setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeGCP)
				h.On("GetRegion").Return("", errors.New("cloud endpoint not reachable"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			h := new(fakeHarvester)
			testCase.setMock(h)
			hostInfo := NewHostInfoCommon(agentTestVersion, true, nil, h)
			data, err := hostInfo.GetHostInfo()
			testCase.assertions(&data, err)
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
			name: "cloud oci",
			assertions: func(tp string, err error) {
				assert.Equal(t, "VM.Optimized3.Flex", tp)
				assert.NoError(t, err)
			}, setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeOCI)
				h.On("GetHostType").Return("VM.Optimized3.Flex", nil)
			},
		},
		{
			name: "cloud error",
			assertions: func(tp string, err error) {
				assert.Equal(t, "unknown", tp)
				assert.Error(t, err)
			}, setMock: func(h *fakeHarvester) {
				h.On("GetCloudType").Return(cloud.TypeAzure)
				// nolint:goerr113
				h.On("GetHostType").Return("", errors.New("endpoint not available"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			h := new(fakeHarvester)
			testCase.setMock(h)
			hostInfo := NewHostInfoCommon("test", true, nil, h)
			testCase.assertions(hostInfo.GetCloudHostType())
			h.AssertExpectations(t)
		})
	}
}
