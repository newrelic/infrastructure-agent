// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
)

// Ref: https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/gettingmetadata.htm

var (
	// ErrOCIRequestFailed indicates failure to prepare OCI metadata request.
	ErrOCIRequestFailed = errors.New("unable to prepare OCI metadata request")
	// ErrOCIFetchFailed indicates failure to fetch OCI metadata.
	ErrOCIFetchFailed = errors.New("unable to fetch OCI metadata")
	// ErrOCIResponseFailed indicates failure due to non-OK response.
	ErrOCIResponseFailed = errors.New("cloud metadata request returned non-OK response")
	// ErrOCIReadFailed indicates failure to read OCI metadata response.
	ErrOCIReadFailed = errors.New("unable to read OCI metadata response body")
	// ErrOCIUnmarshalFailed indicates failure to unmarshal OCI metadata response.
	ErrOCIUnmarshalFailed = errors.New("unable to unmarshal OCI metadata response body")
)

const (
	// ociTimeout is the timeout for OCI metadata requests.
	ociTimeout = 600
	// OciEndpoint is the URL used for requesting OCI metadata.
	ociEndpoint = "http://169.254.169.254/opc/v1/instance/"
)

// OCIHarvester is used to fetch data from OCI api.
type OCIHarvester struct {
	timeout          *Timeout
	disableKeepAlive bool
	instanceID       string // Cache the OCI instance ID.
	hostType         string // Cache the OCI instance Type.
	region           string
	zone             string
	subscriptionID   string
	imageID          string
	vmSize           string
	displayName      string
}

// NewOCIHarvester returns a new instance of OCIHarvester.
func NewOCIHarvester(disableKeepAlive bool) *OCIHarvester {
	return &OCIHarvester{
		timeout:          NewTimeout(ociTimeout),
		disableKeepAlive: disableKeepAlive,
		instanceID:       "",
		hostType:         "",
		region:           "",
		zone:             "",
		subscriptionID:   "",
		imageID:          "",
		vmSize:           "",
		displayName:      "",
	}
}

func (a *OCIHarvester) GetHarvester() (Harvester, error) { //nolint: ireturn
	return a, nil
}

// GetInstanceID returns the OCI instance ID.
func (a *OCIHarvester) GetInstanceID() (string, error) {
	if a.instanceID == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.instanceID = ociMetadata.VMID
	}

	return a.instanceID, nil
}

// GetHostType will return the cloud instance type.
func (a *OCIHarvester) GetHostType() (string, error) {
	if a.hostType == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}

		a.hostType = ociMetadata.VMSize
	}

	return a.hostType, nil
}

// GetCloudType returns the type of the cloud.
func (a *OCIHarvester) GetCloudType() Type {
	return TypeOCI
}

// GetCloudSource returns a string key which will be used as a HostSource (see host_aliases plugin).
func (a *OCIHarvester) GetCloudSource() string {
	return sysinfo.HOST_SOURCE_OCI_VM_ID
}

// GetRegion will return the cloud instance region.
func (a *OCIHarvester) GetRegion() (string, error) {
	if a.region == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}

		a.region = ociMetadata.Location
	}

	return a.region, nil
}

// GetAccountID returns the cloud account.
func (a *OCIHarvester) GetAccountID() (string, error) {
	if a.subscriptionID == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}

		a.subscriptionID = ociMetadata.SubscriptionID
	}

	return a.subscriptionID, nil
}

// GetZone returns the cloud instance zone.
func (a *OCIHarvester) GetZone() (string, error) {
	if a.zone == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.zone = ociMetadata.Zone
	}

	return a.zone, nil
}

// GetInstanceImageID returns the cloud instance image ID.
func (a *OCIHarvester) GetInstanceImageID() (string, error) {
	if a.imageID == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.imageID = ociMetadata.ImageID
	}

	return a.imageID, nil
}

// GetVMSize returns the cloud instance VM size.
func (a *OCIHarvester) GetVMSize() (string, error) {
	if a.vmSize == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.vmSize = ociMetadata.VMSize
	}

	return a.vmSize, nil
}

// GetInstanceDisplayName returns the cloud instance DisplayName.
func (a *OCIHarvester) GetInstanceDisplayName() (string, error) {
	if a.displayName == "" || a.timeout.HasExpired() {
		ociMetadata, err := GetOCIMetadata(a.disableKeepAlive)
		if err != nil {
			return "", err
		}
		a.displayName = ociMetadata.DisplayName
	}

	return a.displayName, nil
}

// OCIMetadata captures the fields we care about from the OCI metadata API.
type OCIMetadata struct {
	Location       string `json:"canonicalRegionName"`
	VMID           string `json:"id"`
	VMSize         string `json:"shape"`
	SubscriptionID string `json:"compartmentId"`
	Zone           string `json:"availabilityDomain"`
	ImageID        string `json:"image"`
	DisplayName    string `json:"displayName"`
}

// GetOCIMetadata is used to request metadata from OCI API.
func GetOCIMetadata(disableKeepAlive bool) (*OCIMetadata, error) {
	var request *http.Request
	var err error

	if request, err = http.NewRequest(http.MethodGet, ociEndpoint, nil); err != nil { //nolint:noctx
		return nil, fmt.Errorf("%w: %w", ErrOCIRequestFailed, err)
	}

	request.Header.Add("Metadata", "true")

	var response *http.Response
	if response, err = clientWithFastTimeout(disableKeepAlive).Do(request); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIFetchFailed, err)
	}
	defer response.Body.Close()

	return parseOCIMetadataResponse(response)
}

// parseOCIMetadataResponse is used to parse the value required from OCI response.
func parseOCIMetadataResponse(response *http.Response) (*OCIMetadata, error) {
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d %s", ErrOCIResponseFailed, response.StatusCode, response.Status)
	}

	var responseBody []byte
	var err error
	if responseBody, err = io.ReadAll(response.Body); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIReadFailed, err)
	}

	var result *OCIMetadata
	if err = json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIUnmarshalFailed, err)
	}

	return result, nil
}
