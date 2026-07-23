// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"fmt"
	"path"
	"runtime"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
)

var clog = log.WithComponent("CloudCommon") //nolint:gochecknoglobals

var (
	// ErrCloudRegionRetrievalFailed indicates failure to retrieve cloud region.
	ErrCloudRegionRetrievalFailed = errors.New("couldn't retrieve cloud region")
	// ErrCloudAccountIDRetrievalFailed indicates failure to retrieve cloud account ID.
	ErrCloudAccountIDRetrievalFailed = errors.New("couldn't retrieve cloud account ID")
	// ErrCloudZoneRetrievalFailed indicates failure to retrieve cloud availability zone.
	ErrCloudZoneRetrievalFailed = errors.New("couldn't retrieve cloud availability zone")
	// ErrCloudImageIDRetrievalFailed indicates failure to retrieve cloud image ID.
	ErrCloudImageIDRetrievalFailed = errors.New("couldn't retrieve cloud image ID")
	// ErrCloudDisplayNameRetrievalFailed indicates failure to retrieve cloud display name.
	ErrCloudDisplayNameRetrievalFailed = errors.New("couldn't retrieve cloud display name")
	// ErrCloudVMSizeRetrievalFailed indicates failure to retrieve cloud VM size.
	ErrCloudVMSizeRetrievalFailed = errors.New("couldn't retrieve cloud VM size")
	// ErrCloudFaultDomainRetrievalFailed indicates failure to retrieve cloud fault domain.
	ErrCloudFaultDomainRetrievalFailed = errors.New("couldn't retrieve cloud fault domain")
	// ErrCloudPrivateDNSNameRetrievalFailed indicates failure to retrieve cloud private DNS name.
	ErrCloudPrivateDNSNameRetrievalFailed = errors.New("couldn't retrieve cloud private DNS name")
	// ErrCloudPrivateIPRetrievalFailed indicates failure to retrieve cloud private IP.
	ErrCloudPrivateIPRetrievalFailed = errors.New("couldn't retrieve cloud private IP")
	// ErrCloudFreeformTagsRetrievalFailed indicates failure to retrieve cloud freeform tags.
	ErrCloudFreeformTagsRetrievalFailed = errors.New("couldn't retrieve cloud freeform tags")
	// ErrCloudInstanceIDRetrievalFailed indicates failure to retrieve the cloud instance ID.
	ErrCloudInstanceIDRetrievalFailed = errors.New("couldn't retrieve cloud instance ID")
)

const (
	// ociCloudProvider is the OTel-aligned cloud.provider value for OCI.
	ociCloudProvider = "oci"
	// ociCloudPlatform is the OTel-aligned cloud.platform value for OCI.
	ociCloudPlatform = "oci_compute"
)

type CloudData struct {
	AwsCloudData     `mapstructure:",squash"`
	AzureCloudData   `mapstructure:",squash"`
	GoogleCloudData  `mapstructure:",squash"`
	AlibabaCloudData `mapstructure:",squash"`
	OracleCloudData  `mapstructure:",squash"`
}

type AwsCloudData struct {
	RegionAWS           string `json:"aws_region,omitempty"`
	AWSAccountID        string `json:"aws_account_id,omitempty"`
	AWSAvailabilityZone string `json:"aws_availability_zone,omitempty"`
	AWSImageID          string `json:"aws_image_id,omitempty"`
}

type AzureCloudData struct {
	RegionAzure           string `json:"region_name,omitempty"`
	AzureSubscriptionID   string `json:"azure_subscription_id,omitempty"`
	AzureAvailabilityZone string `json:"azure_availability_zone,omitempty"`
}

type GoogleCloudData struct {
	RegionGCP string `json:"zone,omitempty"`
}

type AlibabaCloudData struct {
	RegionAlibaba string `json:"region_id,omitempty"`
}

// OracleCloudData holds the OCI attributes emitted as inventory. Field naming follows the
// "CDD: OCI Tag Support to Match AWS Tags in New Relic Infrastructure" (Confluence 5846532426).
type OracleCloudData struct {
	// Phase 1 (IMDS + static values)
	RegionOCI           string `json:"oci.region,omitempty"`
	OCIAccountID        string `json:"oci.compartmentId,omitempty"`
	OCIAvailabilityZone string `json:"oci.availabilityDomain,omitempty"`
	OCIImageID          string `json:"oci.imageId,omitempty"`
	OCIDisplayName      string `json:"displayName,omitempty"`
	OCIVMSize           string `json:"oci.shape,omitempty"`
	CloudResourceID     string `json:"cloud.resource_id,omitempty"`
	OCIInstanceID       string `json:"oci.compute.instanceId,omitempty"`
	OCIFaultDomain      string `json:"oci.compute.faultDomain,omitempty"`
	OCIPrivateIP        string `json:"oci.network.privateIp,omitempty"`
	OCIPrivateDNSName   string `json:"oci.network.privateDnsName,omitempty"`
	HostArch            string `json:"host.arch,omitempty"`
	CloudProvider       string `json:"cloud.provider,omitempty"`
	CloudPlatform       string `json:"cloud.platform,omitempty"`

	// Phase 2 (OCI SDK + instance principal auth). Left empty (not an error) if the OCI API
	// is unreachable or the instance's IAM policy doesn't grant the required read access.
	OCIVCNID              string `json:"oci.network.vcnId,omitempty"`
	OCISubnetID           string `json:"oci.network.subnetId,omitempty"`
	OCILifecycleState     string `json:"oci.compute.lifecycleState,omitempty"`
	OCIVirtualizationType string `json:"oci.compute.virtualizationType,omitempty"`
	OCIDedicatedVMHostID  string `json:"oci.compute.dedicatedVmHostId,omitempty"`

	// OCIFreeformTags holds the (already exclusion-filtered) freeform tags. Excluded from
	// default JSON marshaling: a dynamic key set can't be expressed as struct fields, so it's
	// flattened into "label.<key>" top-level attributes by HostInfoLinux/Darwin/Windows's
	// MarshalJSON via FlattenLabels instead.
	OCIFreeformTags map[string]string `json:"-"`
}

// getAWSCloudData gathers the exported information for the AWS Cloud.
func getAWSCloudData(cloudHarvester cloud.Harvester) (awsData AwsCloudData, err error) {
	awsData.RegionAWS, err = cloudHarvester.GetRegion()
	if err != nil {
		return awsData, fmt.Errorf("couldn't retrieve cloud region: %w", err)
	}

	awsData.AWSImageID, err = cloudHarvester.GetInstanceImageID()
	if err != nil {
		return awsData, fmt.Errorf("couldn't retrieve cloud image ID: %w", err)
	}

	awsData.AWSAccountID, err = cloudHarvester.GetAccountID()
	if err != nil {
		return awsData, fmt.Errorf("couldn't retrieve cloud account ID: %w", err)
	}

	awsData.AWSAvailabilityZone, err = cloudHarvester.GetZone()
	if err != nil {
		return awsData, fmt.Errorf("couldn't retrieve cloud availability zone: %w", err)
	}

	return
}

// getAzureCloudData gathers the exported information for the Azure Cloud.
func getAzureCloudData(cloudHarvester cloud.Harvester) (azureData AzureCloudData, err error) {
	azureData.RegionAzure, err = cloudHarvester.GetRegion()
	if err != nil {
		return azureData, fmt.Errorf("couldn't retrieve cloud region: %w", err)
	}

	azureData.AzureSubscriptionID, err = cloudHarvester.GetAccountID()
	if err != nil {
		return azureData, fmt.Errorf("couldn't retrieve cloud account ID: %w", err)
	}

	azureData.AzureAvailabilityZone, err = cloudHarvester.GetZone()
	if err != nil {
		return azureData, fmt.Errorf("couldn't retrieve cloud availability zone: %w", err)
	}

	return
}

// filterExcludedTags returns a copy of tags with any key matching an exclude pattern removed.
// Patterns support glob syntax via path.Match (e.g. "pipeline-*").
func filterExcludedTags(tags map[string]string, excludePatterns []string) map[string]string {
	if len(tags) == 0 || len(excludePatterns) == 0 {
		return tags
	}

	filtered := make(map[string]string, len(tags))
	for key, value := range tags {
		excluded := false
		for _, pattern := range excludePatterns {
			if matched, err := path.Match(pattern, key); err == nil && matched {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered[key] = value
		}
	}

	return filtered
}

// getOracleCloudData gathers the exported information for the Oracle Cloud.
func getOracleCloudData(cloudHarvester cloud.Harvester, ociTagsExclude []string) (OracleCloudData, error) {
	var ociData OracleCloudData
	var err error

	ociData.RegionOCI, err = cloudHarvester.GetRegion()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudRegionRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIAccountID, err = cloudHarvester.GetAccountID()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudAccountIDRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIAvailabilityZone, err = cloudHarvester.GetZone()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudZoneRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIImageID, err = cloudHarvester.GetInstanceImageID()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudImageIDRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIDisplayName, err = cloudHarvester.GetInstanceDisplayName()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudDisplayNameRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIVMSize, err = cloudHarvester.GetVMSize()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudVMSizeRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIFaultDomain, err = cloudHarvester.GetFaultDomain()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudFaultDomainRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIPrivateDNSName, err = cloudHarvester.GetHostname()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudPrivateDNSNameRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	ociData.OCIPrivateIP, err = cloudHarvester.GetPrivateIP()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudPrivateIPRetrievalFailed.Error(), err) //nolint:wrapcheck
	}

	freeformTags, err := cloudHarvester.GetFreeformTags()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudFreeformTagsRetrievalFailed.Error(), err) //nolint:wrapcheck
	}
	ociData.OCIFreeformTags = filterExcludedTags(freeformTags, ociTagsExclude)

	instanceID, err := cloudHarvester.GetInstanceID()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudInstanceIDRetrievalFailed.Error(), err) //nolint:wrapcheck
	}
	ociData.CloudResourceID = instanceID
	ociData.OCIInstanceID = instanceID

	ociData.HostArch = runtime.GOARCH
	ociData.CloudProvider = ociCloudProvider
	ociData.CloudPlatform = ociCloudPlatform

	// Phase 2 (OCI SDK + instance principal auth): best-effort only. A failure here (missing
	// IAM policy, no instance principal available, etc.) must never block agent startup or
	// prevent Phase 1 attributes from shipping - log and leave the attribute empty instead.
	if ociData.OCIVCNID, err = cloudHarvester.GetVCNID(); err != nil {
		clog.WithError(err).Debug("OCI VCN ID unavailable; leaving oci.network.vcnId empty")
	}
	if ociData.OCISubnetID, err = cloudHarvester.GetSubnetID(); err != nil {
		clog.WithError(err).Debug("OCI subnet ID unavailable; leaving oci.network.subnetId empty")
	}
	if ociData.OCILifecycleState, err = cloudHarvester.GetLifecycleState(); err != nil {
		clog.WithError(err).Debug("OCI lifecycle state unavailable; leaving oci.compute.lifecycleState empty")
	}
	if ociData.OCIVirtualizationType, err = cloudHarvester.GetVirtualizationType(); err != nil {
		clog.WithError(err).Debug("OCI virtualization type unavailable; leaving oci.compute.virtualizationType empty")
	}
	if ociData.OCIDedicatedVMHostID, err = cloudHarvester.GetDedicatedVMHostID(); err != nil {
		clog.WithError(err).Debug("OCI dedicated VM host ID unavailable; leaving oci.compute.dedicatedVmHostId empty")
	}

	return ociData, nil
}

// getCloudData will populate a CloudData structure depending on the cloud type.
func getCloudData(cloudHarvester cloud.Harvester, ociTagsExclude []string) (cloudData CloudData, err error) {
	switch cloudHarvester.GetCloudType() {
	case cloud.TypeAWS:
		cloudData.AwsCloudData, err = getAWSCloudData(cloudHarvester)
	case cloud.TypeAzure:
		cloudData.AzureCloudData, err = getAzureCloudData(cloudHarvester)
	case cloud.TypeGCP:
		cloudData.RegionGCP, err = cloudHarvester.GetRegion()
	case cloud.TypeAlibaba:
		cloudData.RegionAlibaba, err = cloudHarvester.GetRegion()
	case cloud.TypeOCI:
		cloudData.OracleCloudData, err = getOracleCloudData(cloudHarvester, ociTagsExclude)
	case cloud.TypeNoCloud:
		return
	}

	return
}
