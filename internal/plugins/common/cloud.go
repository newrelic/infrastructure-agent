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
	AzureImageID          string `json:"azure_image_id,omitempty"`
	AzureSubscriptionID   string `json:"azure_subscription_id,omitempty"`
	AzureAvailabilityZone string `json:"azure_availability_zone,omitempty"`
}

type GoogleCloudData struct {
	RegionGCP string `json:"zone,omitempty"`
}

type AlibabaCloudData struct {
	RegionAlibaba string `json:"region_id,omitempty"`
}

func GetCloudData(cloudHarvester cloud.Harvester) (CloudData, error) {
	var cloudData CloudData
	var err error
	var imageID, accountID, availabilityZone string

	if cloudHarvester.GetCloudType() == cloud.TypeNoCloud {
		return cloudData, nil
	}

	region, err := cloudHarvester.GetRegion()
	if err != nil {
		return cloudData, fmt.Errorf("couldn't retrieve cloud region: %v", err)
	}

	// Fetch additional cloud metadata only for AWS or Azure clouds
	if cloudHarvester.GetCloudType() == cloud.TypeAWS || cloudHarvester.GetCloudType() == cloud.TypeAzure {
		imageID, err = cloudHarvester.GetInstanceImageID()
		if err != nil {
			return cloudData, fmt.Errorf("couldn't retrieve cloud image ID: %v", err)
		}
		accountID, err = cloudHarvester.GetAccountID()
		if err != nil {
			return cloudData, fmt.Errorf("couldn't retrieve cloud account ID: %v", err)
		}
		availabilityZone, err = cloudHarvester.GetZone()
		if err != nil {
			return cloudData, fmt.Errorf("couldn't retrieve cloud availability zone: %v", err)
		}
	}

	switch cloudHarvester.GetCloudType() {
	case cloud.TypeAWS:
		cloudData.RegionAWS = region
		cloudData.AWSImageID = imageID
		cloudData.AWSAccountID = accountID
		cloudData.AWSAvailabilityZone = availabilityZone
	case cloud.TypeAzure:
		cloudData.RegionAzure = region
		cloudData.AzureImageID = imageID
		cloudData.AzureSubscriptionID = accountID
		cloudData.AzureAvailabilityZone = availabilityZone
	case cloud.TypeGCP:
		cloudData.RegionGCP = region
	case cloud.TypeAlibaba:
		cloudData.RegionAlibaba = region
	}
	return cloudData, nil
}
