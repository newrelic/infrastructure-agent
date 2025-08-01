// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
)

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
	// ErrCloudTenantIDRetrievalFailed indicates failure to retrieve cloud tenant ID.
	ErrCloudTenantIDRetrievalFailed = errors.New("couldn't retrieve cloud tenant ID")
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

type OracleCloudData struct {
	RegionOCI           string `json:"oci_region,omitempty"`
	OCIAccountID        string `json:"oci_account_id,omitempty"`
	OCIAvailabilityZone string `json:"oci_availability_zone,omitempty"`
	OCIImageID          string `json:"oci_image_id,omitempty"`
	OCIDisplayName      string `json:"oci_display_name,omitempty"`
	OCITenantID         string `json:"oci_tenant_id,omitempty"`
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

// getOracleCloudData gathers the exported information for the Oracle Cloud.
func getOracleCloudData(cloudHarvester cloud.Harvester) (OracleCloudData, error) {
	var ociData OracleCloudData
	var err error

	ociData.RegionOCI, err = cloudHarvester.GetRegion()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudRegionRetrievalFailed.Error(), err)
	}

	ociData.OCIAccountID, err = cloudHarvester.GetAccountID()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudAccountIDRetrievalFailed.Error(), err)
	}

	ociData.OCIAvailabilityZone, err = cloudHarvester.GetZone()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudZoneRetrievalFailed.Error(), err)
	}

	ociData.OCIImageID, err = cloudHarvester.GetInstanceImageID()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudImageIDRetrievalFailed.Error(), err)
	}

	ociData.OCIDisplayName, err = cloudHarvester.GetInstanceDisplayName()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudDisplayNameRetrievalFailed.Error(), err)
	}

	ociData.OCITenantID, err = cloudHarvester.GetInstanceTenantID()
	if err != nil {
		return ociData, fmt.Errorf("%s: %w", ErrCloudTenantIDRetrievalFailed.Error(), err)
	}

	return ociData, nil
}

// getCloudData will populate a CloudData structure depending on the cloud type.
func getCloudData(cloudHarvester cloud.Harvester) (cloudData CloudData, err error) {
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
		cloudData.OracleCloudData, err = getOracleCloudData(cloudHarvester)
	case cloud.TypeNoCloud:
		return
	}

	return
}
