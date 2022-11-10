// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
)

type CloudData struct {
	AwsCloudData     `mapstructure:",squash"`
	AzureCloudData   `mapstructure:",squash"`
	GoogleCloudData  `mapstructure:",squash"`
	AlibabaCloudData `mapstructure:",squash"`
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
	case cloud.TypeNoCloud:
		return
	}

	return
}
