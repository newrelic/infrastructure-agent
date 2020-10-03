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

func NewCloudHarvester(cloudId string, cloudType cloud.Type, error bool) *MockCloudHarvester {
	return &MockCloudHarvester{cloudId: cloudId, cloudType: cloudType, error: error,
		ErrorMessage: "error connecting to the cloud API"}
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

// the fingerprint should contain the cloud id if there was no error
func TestCloudNoError(t *testing.T) {
	hostnameResolver := &MockHostNameResolver{}
	cloudHarvester := NewCloudHarvester("i-123", cloud.TypeAWS, false)
	config := config.NewConfig()
	fpHarvester, _ := NewHarvestor(config, hostnameResolver, cloudHarvester)
	fp, err := fpHarvester.Harvest()

	assert.NilError(t, err)
	assert.Equal(t, fp.CloudProviderId, "i-123")
}

// if there was an error and the cloud harvester was initialized - ie. the host was detected to run on the cloud - then
// harvesting the fingerprint should return an error
func TestCloudErrorInitialized(t *testing.T) {
	hostnameResolver := &MockHostNameResolver{}
	cloudHarvester := NewCloudHarvester("", cloud.TypeAWS, true)
	config := config.NewConfig()
	fpHarvester, _ := NewHarvestor(config, hostnameResolver, cloudHarvester)
	fp, err := fpHarvester.Harvest()

	assert.Error(t, err, cloudHarvester.ErrorMessage)
	assert.Equal(t, fp.CloudProviderId, "")
}

// if there was an error but the host does not run on the cloud, then the fingerprint should be created without
// any error
func TestCloudErrorNoCloud(t *testing.T) {
	hostnameResolver := &MockHostNameResolver{}
	cloudHarvester := NewCloudHarvester("", cloud.TypeNoCloud, true)
	config := config.NewConfig()
	fpHarvester, _ := NewHarvestor(config, hostnameResolver, cloudHarvester)
	fp, err := fpHarvester.Harvest()

	assert.NilError(t, err)
	assert.Equal(t, fp.CloudProviderId, "")
}

// if there was an error but we don't know yet whether the host runs on the cloud, then the fingerprint should be
// created without any error
func TestCloudErrorNotInitialized(t *testing.T) {
	hostnameResolver := &MockHostNameResolver{}
	cloudHarvester := NewCloudHarvester("", cloud.TypeInProgress, true)
	config := config.NewConfig()
	fpHarvester, _ := NewHarvestor(config, hostnameResolver, cloudHarvester)
	fp, err := fpHarvester.Harvest()

	assert.NilError(t, err)
	assert.Equal(t, fp.CloudProviderId, "")
}
