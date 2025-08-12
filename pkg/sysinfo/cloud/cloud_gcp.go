// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
)

const (
	// gcpEndpoint is the URL used for requesting GCP metadata.
	gcpEndpoint = "http://metadata.google.internal/computeMetadata/v1/instance/?recursive=true"
)

// GCPHarvester is used to fetch data from GCP API.
type GCPHarvester struct {
	timeout          *Timeout // Interval for re-fetching the data.
	disableKeepAlive bool
	instanceID       string // Cache the gcp instance ID.
	hostType         string // Cache the gcp instance Type.
	zone             string
}

// NewGCPHarvester return a new GCPHarvester instance.
func NewGCPHarvester(disableKeepAlive bool) *GCPHarvester {
	return &GCPHarvester{
		timeout:          NewTimeout(600),
		disableKeepAlive: disableKeepAlive,
	}
}

func (gcp *GCPHarvester) GetHarvester() (Harvester, error) {
	return gcp, nil
}

// GetInstanceID returns the gcp instance ID.
func (gcp *GCPHarvester) GetInstanceID() (string, error) {
	if gcp.instanceID == "" || gcp.timeout.HasExpired() {
		gcpMetadata, err := GetGCPMetadata(gcp.disableKeepAlive)
		if err != nil {
			return "", err
		}
		gcp.instanceID = gcpMetadata.Id
	}

	return gcp.instanceID, nil
}

// GetHostType will return the cloud instance type.
func (gcp *GCPHarvester) GetHostType() (string, error) {
	if gcp.hostType == "" || gcp.timeout.HasExpired() {
		gcpMetadata, err := GetGCPMetadata(gcp.disableKeepAlive)
		if err != nil {
			return "", err
		}
		gcp.hostType = gcpMetadata.MachineType
	}

	return gcp.hostType, nil
}

// GetAccountID returns the cloud account
func (gcp *GCPHarvester) GetAccountID() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetZone returns the cloud instance zone
func (gcp *GCPHarvester) GetZone() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetInstanceImageID returns the cloud instance image ID
func (gcp *GCPHarvester) GetInstanceImageID() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetCloudType returns the type of the cloud.
func (gcp *GCPHarvester) GetCloudType() Type {
	return TypeGCP
}

// GetCloudSource returns a string key which will be used as a HostSource (see host_aliases plugin).
func (gcp *GCPHarvester) GetCloudSource() string {
	return sysinfo.HOST_SOURCE_GCP_VM_ID
}

// GetInstanceDisplayName returns the cloud instance display name (not supported for GCP)
func (gcp *GCPHarvester) GetInstanceDisplayName() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetInstanceTenantID returns the cloud instance tenant ID (not supported for GCP)
func (gcp *GCPHarvester) GetInstanceTenantID() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetRegion will return the cloud instance region.
func (gcp *GCPHarvester) GetRegion() (string, error) {
	if gcp.zone == "" || gcp.timeout.HasExpired() {
		gcpMetadata, err := GetGCPMetadata(gcp.disableKeepAlive)
		if err != nil {
			return "", err
		}
		gcp.zone = gcpMetadata.Zone
	}

	return gcp.zone, nil
}

// Captures the fields we care about from the GCP metadata API.
type gcpMetadata struct {
	Zone        string
	Id          string
	MachineType string
}

// GetGCPMetadata is used to request metadata from GCP API.
func GetGCPMetadata(disableKeepAlive bool) (result *gcpMetadata, err error) {
	var request *http.Request

	if request, err = http.NewRequest(http.MethodGet, gcpEndpoint, nil); err != nil {
		err = fmt.Errorf("unable to prepare GCP metadata request: %v", request)
		return
	}
	request.Header.Add("Metadata-Flavor", "Google")

	var response *http.Response
	if response, err = clientWithFastTimeout(disableKeepAlive).Do(request); err != nil {
		err = fmt.Errorf("unable to fetch GCP metadata: %s", err)
		return
	}
	defer response.Body.Close()

	return parseGCPMetaResponse(response)
}

// parseGCPMetaResponse is used to parse the value required from GCP response.
func parseGCPMetaResponse(response *http.Response) (result *gcpMetadata, err error) {
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("GCP metadata request returned non-OK response: %d %s", response.StatusCode, response.Status)
		return
	}

	var responseBody []byte
	if responseBody, err = ioutil.ReadAll(response.Body); err != nil {
		err = fmt.Errorf("unable to read GCP metadata response body: %v", err)
		return
	}

	// Intermediate representation of metadata before curating it
	tmpRep := struct {
		Zone        string      `json:"zone"`
		Id          json.Number `json:"id,Number"`
		MachineType string      `json:"machineType"`
	}{}

	if err = json.Unmarshal(responseBody, &tmpRep); err != nil {
		err = fmt.Errorf("unable to unmarshal GCP metadata response body: %v", err)
		return
	}

	result = &gcpMetadata{
		Zone:        path.Base(tmpRep.Zone),
		Id:          "gcp-" + string(tmpRep.Id),
		MachineType: path.Base(tmpRep.MachineType),
	}

	return
}
