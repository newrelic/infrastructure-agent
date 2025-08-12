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
	// alibabaEndpoint is the URL used for requesting Alibaba metadata.
	alibabaEndpoint = "http://100.100.100.200/latest/dynamic/instance-identity/document"
)

//
// https://www.alibabacloud.com/help/doc-detail/49122.htm
//
// This response contains metadata about the alibaba instance.
// Example response:
//
// {
//     "zone-id": "ap-southeast-2b",
//     "serial-number": "d54da90b-fdde-46a9-bb0b-703946c53411",
//     "instance-id": "i-p0we7kj126dhd52fh5w8",
//     "region-id": "ap-southeast-2",
//     "private-ipv4": "172.27.17.70",
//     "owner-account-id": "5075089599391873",
//     "mac": "00:16:3e:00:0f:de",
//     "image-id": "ubuntu_18_04_64_20G_alibase_20190223.vhd",
//     "instance-type": "ecs.t5-lc1m1.small"
// }
//

// AlibabaHarvester is used to fetch data from Alibaba api.
type AlibabaHarvester struct {
	timeout          *Timeout
	disableKeepAlive bool
	instanceID       string // Cache the Alibaba instance ID.
	hostType         string // Cache the Alibaba instance Type.
	region           string
	zone             string // Cache the Alibaba instance Zone.
	instanceImageID  string // Cache the Alibaba instance Image ID.
	account          string // Cache the Alibaba account ID.
}

// AlibabaHarvester returns a new instance of AlibabaHarvester.
func NewAlibabaHarvester(disableKeepAlive bool) *AlibabaHarvester {
	return &AlibabaHarvester{
		timeout:          NewTimeout(600),
		disableKeepAlive: disableKeepAlive,
	}
}

// GetHarvester returns instance of the Harvester detected (or instance of themselves)
func (a *AlibabaHarvester) GetHarvester() (Harvester, error) {
	return a, nil
}

// GetInstanceID returns the Alibaba instance ID.
func (a *AlibabaHarvester) GetInstanceID() (string, error) {
	if a.instanceID == "" || a.timeout.HasExpired() {
		AlibabaMetadata, err := GetAlibabaMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.instanceID = AlibabaMetadata.InstanceID
	}

	return a.instanceID, nil
}

// GetHostType will return the cloud instance type.
func (a *AlibabaHarvester) GetHostType() (string, error) {
	if a.hostType == "" || a.timeout.HasExpired() {
		AlibabaMetadata, err := GetAlibabaMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.hostType = AlibabaMetadata.InstanceType
	}

	return a.hostType, nil
}

// GetCloudType returns the type of the cloud.
func (a *AlibabaHarvester) GetCloudType() Type {
	return TypeAlibaba
}

// GetCloudSource returns a string key which will be used as a HostSource (see host_aliases plugin).
func (a *AlibabaHarvester) GetCloudSource() string {
	return sysinfo.HOST_SOURCE_ALIBABA_VM_ID
}

// GetInstanceDisplayName returns the cloud instance display name (not supported for Alibaba)
func (a *AlibabaHarvester) GetInstanceDisplayName() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetInstanceTenantID returns the cloud instance tenant ID (not supported for Alibaba)
func (a *AlibabaHarvester) GetInstanceTenantID() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetRegion will return the cloud instance region.
func (a *AlibabaHarvester) GetRegion() (string, error) {
	if a.region == "" || a.timeout.HasExpired() {
		AlibabaMetadata, err := GetAlibabaMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.region = AlibabaMetadata.RegionID
	}

	return a.region, nil
}

// GetAccount will return the cloud account.
func (a *AlibabaHarvester) GetAccountID() (string, error) {
	if a.account == "" || a.timeout.HasExpired() {
		AlibabaMetadata, err := GetAlibabaMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.account = AlibabaMetadata.AccountID
	}

	return a.account, nil
}

// GetAvailability will return the cloud availability zone.
func (a *AlibabaHarvester) GetZone() (string, error) {
	if a.zone == "" || a.timeout.HasExpired() {
		AlibabaMetadata, err := GetAlibabaMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.zone = AlibabaMetadata.Zone
	}

	return a.zone, nil
}

// GetImageID will return the cloud image ID.
func (a *AlibabaHarvester) GetInstanceImageID() (string, error) {
	if a.instanceImageID == "" || a.timeout.HasExpired() {
		AlibabaMetadata, err := GetAlibabaMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.instanceImageID = AlibabaMetadata.InstanceImageID
	}

	return a.instanceImageID, nil
}

// Captures the fields we care about from the Alibaba metadata API
type AlibabaMetadata struct {
	RegionID        string `json:"region-id"`
	InstanceID      string `json:"instance-id"`
	InstanceType    string `json:"instance-type"`
	InstanceImageID string `json:"image-id"`
	AccountID       string `json:"owner-account-id"`
	Zone            string `json:"zone-id"`
}

// GetAlibabaMetadata is used to request metadata from Alibaba API.
func GetAlibabaMetadata(disableKeepAlive bool) (result *AlibabaMetadata, err error) {
	var request *http.Request
	if request, err = http.NewRequest(http.MethodGet, alibabaEndpoint, nil); err != nil {
		err = fmt.Errorf("unable to prepare Alibaba metadata request: %v", request)
		return
	}

	var response *http.Response
	if response, err = clientWithFastTimeout(disableKeepAlive).Do(request); err != nil {
		err = fmt.Errorf("unable to fetch Alibaba metadata: %s", err)
		return
	}
	defer response.Body.Close()

	return parseAlibabaMetadataResponse(response)
}

// parseAlibabaMetadataResponse is used to parse the value required from Alibaba response.
func parseAlibabaMetadataResponse(response *http.Response) (result *AlibabaMetadata, err error) {
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("cloud metadata request returned non-OK response: %d %s", response.StatusCode, response.Status)
		return
	}

	var responseBody []byte
	if responseBody, err = ioutil.ReadAll(response.Body); err != nil {
		err = fmt.Errorf("unable to read Alibaba metadata response body: %v", err)
		return
	}

	if err = json.Unmarshal(responseBody, &result); err != nil {
		err = fmt.Errorf("unable to unmarshal Alibaba metadata response body: %v", err)
		return
	}

	return
}
