// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

type AwsCloudData struct {
	RegionAWS           string `json:"aws_region,omitempty"`
	AWSAccountID        string `json:"aws_account_id,omitempty"`
	AWSAvailabilityZone string `json:"aws_availability_zone,omitempty"`
	AWSImageID          string `json:"aws_image_id,omitempty"`
}

type AzureCloudData struct {
	RegionAzure string `json:"region_name,omitempty"`
}

type GoogleCloudData struct {
	RegionGCP string `json:"zone,omitempty"`
}

type AlibabaCloudData struct {
	RegionAlibaba string `json:"region_id,omitempty"`
}
