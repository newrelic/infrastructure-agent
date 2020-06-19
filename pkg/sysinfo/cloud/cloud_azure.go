// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
)

const (
	// azureEndpoint is the URL used for requesting Azure metadata.
	azureEndpoint = "http://169.254.169.254/metadata/instance?api-version=2017-04-02"
)

// AzureHarvester is used to fetch data from Azure api.
type AzureHarvester struct {
	timeout          *Timeout
	disableKeepAlive bool
	instanceID       string // Cache the azure instance ID.
	hostType         string // Cache the azure instance Type.
	region           string
}

// AzureHarvester returns a new instance of AzureHarvester.
func NewAzureHarvester(disableKeepAlive bool) *AzureHarvester {
	return &AzureHarvester{
		timeout:          NewTimeout(600),
		disableKeepAlive: disableKeepAlive,
	}
}

func (a *AzureHarvester) GetHarvester() (Harvester, error) {
	return a, nil
}

// GetInstanceID returns the Azure instance ID.
func (a *AzureHarvester) GetInstanceID() (string, error) {
	if a.instanceID == "" || a.timeout.HasExpired() {
		azureMetadata, err := GetAzureMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.instanceID = azureMetadata.Compute.VmId
	}

	return a.instanceID, nil
}

// GetHostType will return the cloud instance type.
func (a *AzureHarvester) GetHostType() (string, error) {
	if a.hostType == "" || a.timeout.HasExpired() {
		azureMetadata, err := GetAzureMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.hostType = azureMetadata.Compute.VmSize
	}

	return a.hostType, nil
}

// GetCloudType returns the type of the cloud.
func (a *AzureHarvester) GetCloudType() Type {
	return TypeAzure
}

// GetCloudSource returns a string key which will be used as a HostSource (see host_aliases plugin).
func (a *AzureHarvester) GetCloudSource() string {
	return sysinfo.HOST_SOURCE_AZURE_VM_ID
}

// GetRegion will return the cloud instance region.
func (a *AzureHarvester) GetRegion() (string, error) {
	if a.region == "" || a.timeout.HasExpired() {
		azureMetadata, err := GetAzureMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.region = azureMetadata.Compute.Location
	}

	return a.region, nil
}

// Captures the fields we care about from the Azure metadata API
type azureMetadata struct {
	Compute struct {
		Location string `json:"location"`
		VmId     string `json:"vmId"`
		VmSize   string `json:"vmSize"`
	} `json:"compute"`
}

// GetAzureMetadata is used to request metadata from Azure API.
func GetAzureMetadata(disableKeepAlive bool) (result *azureMetadata, err error) {
	var request *http.Request
	if request, err = http.NewRequest(http.MethodGet, azureEndpoint, nil); err != nil {
		err = fmt.Errorf("unable to prepare Azure metadata request: %v", request)
		return
	}
	request.Header.Add("Metadata", "true")

	var response *http.Response
	if response, err = clientWithFastTimeout(disableKeepAlive).Do(request); err != nil {
		err = fmt.Errorf("unable to fetch Azure metadata: %s", err)
		return
	}
	defer response.Body.Close()

	return parseAzureMetadataResponse(response)
}

// parseAzureMetadataResponse is used to parse the value required from Azure response.
func parseAzureMetadataResponse(response *http.Response) (result *azureMetadata, err error) {
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("cloud metadata request returned non-OK response: %d %s", response.StatusCode, response.Status)
		return
	}

	var responseBody []byte
	if responseBody, err = ioutil.ReadAll(response.Body); err != nil {
		err = fmt.Errorf("unable to read Azure metadata response body: %v", err)
		return
	}

	if err = json.Unmarshal(responseBody, &result); err != nil {
		err = fmt.Errorf("unable to unmarshal Azure metadata response body: %v", err)
		return
	}

	return
}
