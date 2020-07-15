// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type CloudDetectionSuite struct{}

var _ = Suite(&CloudDetectionSuite{})

func (s *CloudDetectionSuite) TestParseAWSMeta(c *C) {
	mux := http.NewServeMux()
	mux.Handle("/valid", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("i-db519dd1\n"))
		return
	}))
	mux.Handle("/list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("foo\nbar"))
		return
	}))
	mux.Handle("/not200", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("foo"))
		return
	}))
	mux.Handle("/notplain", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("foo"))
		return
	}))
	mux.Handle("/justgarbage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>this is some test</html>"))
		return
	}))
	server := httptest.NewServer(mux)

	endpointsAndResults := map[string]string{
		"valid":       "i-db519dd1",
		"list":        "foo\nbar",
		"not200":      "error",
		"notplain":    "error",
		"justgarbage": "error",
	}

	for uri, expectedResult := range endpointsAndResults {
		url := fmt.Sprintf("%s/%s", server.URL, uri)
		resp, err := http.Get(url)
		c.Assert(err, IsNil)
		result, err := parseAWSMetaResponse(resp)
		if expectedResult == "error" {
			c.Check(err, Not(IsNil))
		} else {
			c.Check(result, Equals, expectedResult)
		}
	}
}

func (s *CloudDetectionSuite) TestParseAzureMetaGoldenPath(c *C) {
	response := &http.Response{
		StatusCode: 200,
		Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`{
      "compute": {
        "location": "eastus",
        "name": "mwagner-test-linux-08082017",
        "offer": "UbuntuServer",
        "osType": "Linux",
        "platformFaultDomain": "0",
        "platformUpdateDomain": "0",
        "publisher": "Canonical",
        "sku": "16.04-LTS",
        "version": "16.04.201708030",
        "vmId": "67122ba9-ec37-4029-b1d6-d1ddeca0a64d",
        "vmSize": "Standard_DS1_v2"
      },
      "network": {
        "interface": [
          {
            "ipv4": {
              "ipAddress": [
                {
                  "privateIpAddress": "10.0.1.5",
                  "publicIpAddress": "52.168.11.86"
                }
              ],
              "subnet": [
                {
                  "address": "10.0.1.0",
                  "prefix": "24"
                }
              ]
            },
            "ipv6": {
              "ipAddress": [
              ]
            },
            "macAddress": "000D3A10747F"
          }
        ]
      }
    }`))),
	}

	metadata, err := parseAzureMetadataResponse(response)
	c.Assert(metadata, NotNil)
	c.Assert(err, IsNil)
	c.Assert(metadata.Compute.VmId, Equals, "67122ba9-ec37-4029-b1d6-d1ddeca0a64d")
	c.Assert(metadata.Compute.VmSize, Equals, "Standard_DS1_v2")
	c.Assert(metadata.Compute.Location, Equals, "eastus")
}

func (s *CloudDetectionSuite) TestParseAzureMeta404(c *C) {
	response := &http.Response{
		StatusCode: 404,
		Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`Not Found`))),
	}

	metadata, err := parseAzureMetadataResponse(response)
	c.Assert(metadata, IsNil)
	c.Assert(err, NotNil)
}

func (s *CloudDetectionSuite) TestParseAzureMetaBadFormat(c *C) {
	response := &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`Unexpected Data`))),
	}

	metadata, err := parseAzureMetadataResponse(response)
	c.Assert(metadata, IsNil)
	c.Assert(err, NotNil)
}

func (s *CloudDetectionSuite) TestParseGCPMeta(c *C) {
	response := &http.Response{
		StatusCode: 200,
		Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`{
		"attributes": {},
		"cpuPlatform": "Intel Haswell",
		"description": "",
		"disks": [
			{
				"deviceName": "mmacias-micro",
				"index": 0,
				"mode": "READ_WRITE",
				"type": "PERSISTENT"
			}
		],
		"hostname": "mmacias-micro.c.beyond-181918.internal",
		"id": 6331980990053453154,
		"image": "projects/debian-cloud/global/images/debian-9-stretch-v20171025",
		"licenses": [
			{
				"id": "1000205"
			}
		],
		"machineType": "projects/260890654058/machineTypes/f1-micro",
		"maintenanceEvent": "NONE",
		"name": "mmacias-micro",
		"networkInterfaces": [
			{
				"accessConfigs": [
					{
						"externalIp": "104.154.137.202",
						"type": "ONE_TO_ONE_NAT"
					}
				],
				"forwardedIps": [],
				"ip": "10.128.0.5",
				"ipAliases": [],
				"mac": "42:01:0a:80:00:05",
				"network": "projects/260890654058/networks/default",
				"targetInstanceIps": []
			}
		],
		"preempted": "FALSE",
		"scheduling": {
			"automaticRestart": "TRUE",
			"onHostMaintenance": "MIGRATE",
			"preemptible": "FALSE"
		},
		"serviceAccounts": {},
		"tags": [],
		"virtualClock": {
			"driftToken": "0"
		},
		"zone": "projects/260890654058/zones/us-central1-c"
	}`))),
	}

	metadata, err := parseGCPMetaResponse(response)
	c.Assert(metadata, NotNil)
	c.Assert(err, IsNil)
	c.Assert(metadata.Id, Equals, "gcp-6331980990053453154")
	c.Assert(metadata.MachineType, Equals, "f1-micro")
	c.Assert(metadata.Zone, Equals, "us-central1-c")
}

func (s *CloudDetectionSuite) TestParseGCPMeta404(c *C) {
	response := &http.Response{
		StatusCode: 404,
		Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`Not Found`))),
	}

	metadata, err := parseGCPMetaResponse(response)
	c.Assert(metadata, IsNil)
	c.Assert(err, NotNil)
}

func (s *CloudDetectionSuite) TestParseGCPMetaBadFormat(c *C) {
	response := &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBuffer([]byte(`Unexpected Data`))),
	}

	metadata, err := parseGCPMetaResponse(response)
	c.Assert(metadata, IsNil)
	c.Assert(err, NotNil)
}

type MockHarvester struct {
	mockType   Type
	retryCount int
}

func NewMockHarvester(mockType Type) *MockHarvester {
	return &MockHarvester{
		mockType: mockType,
	}
}

// GetInstanceID will return the id of the cloud instance.
func (m *MockHarvester) GetInstanceID() (string, error) {
	m.retryCount++

	if m.mockType == TypeGCP && m.retryCount == 2 {
		return strconv.Itoa(m.retryCount), nil
	}

	return "", fmt.Errorf("an unexpected error")
}

// GetHostType will return the cloud instance type.
func (m *MockHarvester) GetHostType() (string, error) {
	return "test host type", nil
}

// GetCloudType will return the cloud type on which the instance is running.
func (m *MockHarvester) GetCloudType() Type {
	return m.mockType
}

// Returns a string key which will be used as a HostSource (see host_aliases plugin).
func (m *MockHarvester) GetCloudSource() string {
	return "test cloud source"
}

// GetRegion returns the region where the instance is running.
func (m *MockHarvester) GetRegion() (string, error) {
	return "myRegion", nil
}

// GetHarvester returns the MockHarvester
func (m *MockHarvester) GetHarvester() (Harvester, error) {
	return m, nil
}

func (s *CloudDetectionSuite) TestDetectSuccessful(c *C) {
	detector := NewDetector(false, 10, 0, 0, false)

	gcpHarvester := NewMockHarvester(TypeGCP)
	awsHarvester := NewMockHarvester(TypeAWS)
	azureHarvester := NewMockHarvester(TypeAzure)
	alibabaHarvester := NewMockHarvester(TypeAlibaba)

	done := make(chan struct{})
	go func() {
		detector.initialize(gcpHarvester, awsHarvester, azureHarvester, alibabaHarvester)
		for {
			if detector.isInitialized() {
				done <- struct{}{}
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		close(done)
	}

	c.Assert(detector.isInitialized(), Equals, true)
	c.Assert(detector.getHarvester(), Equals, gcpHarvester)
	c.Assert(detector.GetCloudType(), Equals, TypeGCP)
	c.Assert(gcpHarvester.retryCount, Equals, 2)
}

func (s *CloudDetectionSuite) TestDetectFail(c *C) {
	detector := NewDetector(false, 10, 0, 0, false)

	awsHarvester := NewMockHarvester(TypeAWS)
	azureHarvester := NewMockHarvester(TypeAzure)
	alibabaHarvester := NewMockHarvester(TypeAlibaba)

	done := make(chan struct{})
	go func() {
		detector.initialize(awsHarvester, azureHarvester, alibabaHarvester)
		for {
			if detector.isInitialized() {
				done <- struct{}{}
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		close(done)
	}

	c.Assert(detector.isInitialized(), Equals, true)
	c.Assert(detector.getHarvester(), Equals, nil)
	c.Assert(awsHarvester.retryCount, Equals, 11) // 1 initial detection + 10 retries in background.

	c.Assert(detector.GetCloudType(), Equals, TypeNoCloud)
}

func pseudoSleep(t *Timeout, period time.Duration) {
	t.expiry = t.expiry.Add(-period)
}
func (s *CloudDetectionSuite) TestTimeout(c *C) {
	timeout := NewTimeout(60)
	c.Assert(timeout.HasExpired(), Equals, false)
	pseudoSleep(timeout, 1*time.Second)
	c.Assert(timeout.HasExpired(), Equals, false)
	pseudoSleep(timeout, 60*time.Second)
	c.Assert(timeout.HasExpired(), Equals, true)
	c.Assert(timeout.HasExpired(), Equals, false)
	pseudoSleep(timeout, 61*time.Second)
	c.Assert(timeout.HasExpired(), Equals, true)
}

func TestClientResponseDisableKeepAlive(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.EqualValues(t, "close", r.Header.Get("Connection"))
	}))
	defer ts.Close()

	request, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	assert.NoError(t, err)

	response, err := clientWithFastTimeout(true).Do(request)
	assert.NoError(t, err)
	defer response.Body.Close()
}
