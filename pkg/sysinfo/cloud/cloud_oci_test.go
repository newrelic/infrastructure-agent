// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// output generated with: curl -s http://169.254.169.254/opc/v1/instance/
func TestParseOCIMetadataResponse(t *testing.T) {
	t.Parallel()

	jsonBody := `{
		"availabilityDomain": "jyDh:US-ASHBURN-AD-1",
		"canonicalRegionName": "us-ashburn-1",
		"compartmentId": "ocid1.compartment.oc1",
		"definedTags": {
			"Oracle-Tags": {
				"CreatedBy": "hidden",
				"CreatedOn": "hidden"
			}
		},
		"displayName": "ubuntu-instance-20250722-1328",
		"faultDomain": "hidden",
		"hostname": "hidden",
		"id": "ocid1.instance.oc1",
		"image": "ocid1.image.oc1",
		"metadata": {
			"ssh_authorized_keys": "hidden"
		},
		"ociAdName": "hidden",
		"region": "hidden",
		"regionInfo": {
			"realmDomainComponent": "hidden",
			"realmKey": "hidden",
			"regionIdentifier": "hidden",
			"regionKey": "hidden"
		},
		"shape": "VM.Optimized3.Flex",
		"shapeConfig": {
			"maxVnicAttachments": 2,
			"memoryInGBs": 14.0,
			"networkingBandwidthInGbps": 4.0,
			"ocpus": 1.0
		},
		"state": "hidden",
		"timeCreated": 1753171691822
	}`

	response := &http.Response{ //nolint:exhaustruct
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(jsonBody)),
	}

	metadata, err := parseOCIMetadataResponse(response)
	require.NoError(t, err)
	require.Equal(t, "us-ashburn-1", metadata.Location)
	require.Equal(t, "ocid1.instance.oc1", metadata.VMID)
	require.Equal(t, "VM.Optimized3.Flex", metadata.VMSize)
	require.Equal(t, "ocid1.compartment.oc1", metadata.SubscriptionID)
	require.Equal(t, "jyDh:US-ASHBURN-AD-1", metadata.Zone)
	require.Equal(t, "ocid1.image.oc1", metadata.ImageID)
	require.Equal(t, "ubuntu-instance-20250722-1328", metadata.DisplayName)
}
