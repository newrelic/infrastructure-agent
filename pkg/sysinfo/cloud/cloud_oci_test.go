// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// output generated with: curl -s http://169.254.169.254/opc/v1/instance/
func TestParseOCIMetadataResponse(t *testing.T) {
	response := &http.Response{
		StatusCode: 200,
		Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`{
			"availabilityDomain": "jyDh:US-ASHBURN-AD-1",
			"canonicalRegionName": "us-ashburn-1",
			"compartmentId": "ocid1.compartment.oc1",
			"definedTags": {
				"Oracle-Tags": {
					"CreatedBy": "<hidden>",
					"CreatedOn": "<hidden>"
				}
			},
			"displayName": "ubunut-instance-20250722-1328",
			"faultDomain": "<hidden>",
			"hostname": "<hidden>",
			"id": "ocid1.instance.oc1",
			"image": "ocid1.image.oc1",
			"metadata": {
				"ssh_authorized_keys": "<hidden>"
			},
			"ociAdName": "<hidden>",
			"region": "<hidden>",
			"regionInfo": {
				"realmDomainComponent": "<hidden>",
				"realmKey": "<hidden>",
				"regionIdentifier": "<hidden>",
				"regionKey": "<hidden>"
			},
			"shape": "VM.Optimized3.Flex",
			"shapeConfig": {
				"maxVnicAttachments": 2,
				"memoryInGBs": 14.0,
				"networkingBandwidthInGbps": 4.0,
				"ocpus": 1.0
			},
			"state": "<hidden>",
			"tenantId": "ocid1.tenancy.oc1",
			"timeCreated": 1753171691822
	}`))),
	}

	metadata, err := parseOCIMetadataResponse(response)
	assert.NoError(t, err)
	assert.Equal(t, metadata.Location, "us-ashburn-1")
	assert.Equal(t, metadata.VMID, "ocid1.instance.oc1")
	assert.Equal(t, metadata.VMSize, "VM.Optimized3.Flex")
	assert.Equal(t, metadata.SubscriptionID, "ocid1.compartment.oc1")
	assert.Equal(t, metadata.Zone, "jyDh:US-ASHBURN-AD-1")
	assert.Equal(t, metadata.ImageID, "ocid1.image.oc1")
	assert.Equal(t, metadata.TenantID, "ocid1.tenancy.oc1")
	assert.Equal(t, metadata.DisplayName, "ubunut-instance-20250722-1328")
}
