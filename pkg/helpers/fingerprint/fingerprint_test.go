package fingerprint

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"gotest.tools/assert"
	"testing"
)

type MockHostNameResolver struct{}

func (r *MockHostNameResolver) Query() (full, short string, err error) {
	return "full", "short", nil
}

func (r *MockHostNameResolver) Long() (short string) {
	return "long"
}

type MockCloudHarvester struct {
	cloudId   string
	cloudType cloud.Type
	// if true, it means calls to the cloud APIs will return an error
	error        bool
	ErrorMessage string
}

const CloudErrorMessage = "error connecting to the cloud API"

func NewCloudHarvester(cloudId string, cloudType cloud.Type, error bool) *MockCloudHarvester {
	return &MockCloudHarvester{cloudId: cloudId, cloudType: cloudType, error: error,
		ErrorMessage: CloudErrorMessage}
}

func (a *MockCloudHarvester) GetInstanceID() (string, error) {
	if a.error {
		return "", fmt.Errorf(a.ErrorMessage)
	}
	return a.cloudId, nil
}

func (a *MockCloudHarvester) GetHostType() (string, error) {
	return "", nil
}

func (a *MockCloudHarvester) GetCloudType() cloud.Type {
	return a.cloudType
}

func (a *MockCloudHarvester) GetCloudSource() string {
	return ""
}

func (a *MockCloudHarvester) GetRegion() (string, error) {
	return "", nil
}

func (a *MockCloudHarvester) GetHarvester() (cloud.Harvester, error) {
	return nil, nil
}

func TestCloud(t *testing.T) {
	hostnameResolver := &MockHostNameResolver{}
	config := config.NewConfig()

	var tests = []struct {
		testName           string
		cloudHarvester     cloud.Harvester
		expectedInstanceId string
		expectedError      error
	}{
		// the fingerprint should contain the cloud id if there was no error
		{"NoError", NewCloudHarvester("i-123", cloud.TypeAWS, false), "i-123", nil},
		// if there was an error and the cloud harvester was initialized - ie. the host was detected to run on the cloud - then
		// harvesting the fingerprint should return an error
		{"ErrorCloud", NewCloudHarvester("", cloud.TypeAWS, true), "", fmt.Errorf(CloudErrorMessage)},
		// if there was an error but the host does not run on the cloud, then the fingerprint should be created without
		// any error
		{"ErrorNotcloud", NewCloudHarvester("", cloud.TypeNoCloud, true), "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			fpHarvester, _ := NewHarvestor(config, hostnameResolver, tt.cloudHarvester)
			fp, err := fpHarvester.Harvest()
			assert.Equal(t, fp.CloudProviderId, tt.expectedInstanceId)
			if tt.expectedError != nil {
				assert.Error(t, err, tt.expectedError.Error())
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
