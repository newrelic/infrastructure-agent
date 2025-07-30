// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
)

// Metadata sample: https://docs.microsoft.com/en-us/oci/virtual-machines/linux/instance-metadata-service?tabs=linux
// API versions: https://learn.microsoft.com/en-us/oci/virtual-machines/windows/instance-metadata-service?tabs=windows#endpoint-categories

const (
	// OciEndpoint is the URL used for requesting Oci metadata.
	ociEndpoint = "http://169.254.169.254/opc/v1/instance/"
)

// OciHarvester is used to fetch data from Oci api.
type OciHarvester struct {
	timeout          *Timeout
	disableKeepAlive bool
	instanceID       string // Cache the oci instance ID.
	hostType         string // Cache the oci instance Type.
	region           string
	zone             string
	subscriptionID   string
	imageID          string
	tenantID         string
	displayName      string
}

// OciHarvester returns a new instance of OciHarvester.
func NewOciHarvester(disableKeepAlive bool) *OciHarvester {
	return &OciHarvester{
		timeout:          NewTimeout(600),
		disableKeepAlive: disableKeepAlive,
	}
}

func (a *OciHarvester) GetHarvester() (Harvester, error) {
	return a, nil
}

// GetInstanceID returns the Oci instance ID.
func (a *OciHarvester) GetInstanceID() (string, error) {
	//a.instanceID = "ocid1.instance.oc1.iad.anuwcljrtvlqdbycqmaxcv7d7muhpawcuwywgz6mqc7ipjyafo6dswjlmqca"
	if a.instanceID == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.instanceID = ociMetadata.VmId
	}

	return a.instanceID, nil
}

// GetHostType will return the cloud instance type.
func (a *OciHarvester) GetHostType() (string, error) {
	//a.hostType = "VM.Optimized3.Flex"
	if a.hostType == "" || a.timeout.HasExpired() {
		OciMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}

		a.hostType = OciMetadata.VmSize
	}

	return a.hostType, nil
}

// GetCloudType returns the type of the cloud.
func (a *OciHarvester) GetCloudType() Type {
	return TypeOci
}

// GetCloudSource returns a string key which will be used as a HostSource (see host_aliases plugin).
func (a *OciHarvester) GetCloudSource() string {
	return sysinfo.HOST_SOURCE_OCI_VM_ID
}

// GetRegion will return the cloud instance region.
func (a *OciHarvester) GetRegion() (string, error) {
	//a.region = "iad"
	if a.region == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}

		a.region = ociMetadata.Location
	}

	return a.region, nil
}

// GetAccountID returns the cloud account
func (a *OciHarvester) GetAccountID() (string, error) {
	//a.subscriptionID = "ocid1.compartment.oc1..aaaaaaaad544qzjef2rhbitefrir7rekhrxn6tgc2ycms2nyzooh4nydp4uq"
	if a.subscriptionID == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}

		a.subscriptionID = ociMetadata.SubscriptionID
	}

	return a.subscriptionID, nil
}

// GetZone returns the cloud instance zone
func (a *OciHarvester) GetZone() (string, error) {
	//a.zone = "jyDh:US-ASHBURN-AD-1"
	if a.zone == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.zone = ociMetadata.Zone
	}

	return a.zone, nil
}

// GetInstanceImageID returns the cloud instance image ID
func (a *OciHarvester) GetInstanceImageID() (string, error) {
	//a.imageID = "ocid1.image.oc1.iad.aaaaaaaaylsmeurrokhxpgd2kg6akdd2qkuoryzauxart5ruowwgn3gpaxua"
	if a.imageID == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.imageID = ociMetadata.ImageID
	}

	return a.imageID, nil
}

// GetInstanceTenantID returns the cloud instance Tenant ID
func (a *OciHarvester) GetInstanceTenantID() (string, error) {
	if a.tenantID == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.tenantID = ociMetadata.TenantID
	}

	return a.tenantID, nil
}

// GetInstanceDisplayName returns the cloud instance DisplayName
func (a *OciHarvester) GetInstanceDisplayName() (string, error) {
	if a.displayName == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOciMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.displayName = ociMetadata.DisplayName
	}

	return a.displayName, nil
}

// Captures the fields we care about from the Oci metadata API
type ociMetadata struct {
	Location       string `json:"canonicalRegionName"`
	VmId           string `json:"id"`
	VmSize         string `json:"shape"`
	SubscriptionID string `json:"compartmentId"`
	Zone           string `json:"availabilityDomain"`
	ImageID        string `json:"image"`
	TenantID       string `json:"tenantId"`
	DisplayName    string `json:"displayName"`
}

// GetOciMetadata is used to request metadata from Oci API.
func GetOciMetadata(disableKeepAlive bool) (result *ociMetadata, err error) {
	var request *http.Request
	if request, err = http.NewRequest(http.MethodGet, ociEndpoint, nil); err != nil {
		err = fmt.Errorf("unable to prepare Oci metadata request: %v", request)
		return
	}
	request.Header.Add("Metadata", "true")

	var response *http.Response
	if response, err = clientWithFastTimeout(disableKeepAlive).Do(request); err != nil {
		err = fmt.Errorf("unable to fetch Oci metadata: %s", err)
		return
	}
	defer response.Body.Close()

	return parseOciMetadataResponse(response)
}

// parseOciMetadataResponse is used to parse the value required from Oci response.
func parseOciMetadataResponse(response *http.Response) (result *ociMetadata, err error) {
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("cloud metadata request returned non-OK response: %d %s", response.StatusCode, response.Status)
		return
	}

	var responseBody []byte
	if responseBody, err = io.ReadAll(response.Body); err != nil {
		err = fmt.Errorf("unable to read Oci metadata response body: %v", err)
		return
	}

	if err = json.Unmarshal(responseBody, &result); err != nil {
		err = fmt.Errorf("unable to unmarshal Oci metadata response body: %v", err)
		return
	}

	return
}
