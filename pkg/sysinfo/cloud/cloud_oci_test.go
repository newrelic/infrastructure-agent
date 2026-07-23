// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// output generated with: curl -s -H "Authorization: Bearer Oracle" http://169.254.169.254/opc/v2/instance/
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
		"faultDomain": "FAULT-DOMAIN-1",
		"freeformTags": {
			"env": "prod",
			"team": "infra"
		},
		"hostname": "ubuntu-instance-20250722-1328",
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
	require.Equal(t, "FAULT-DOMAIN-1", metadata.FaultDomain)
	require.Equal(t, "ubuntu-instance-20250722-1328", metadata.Hostname)
	require.Equal(t, map[string]string{"env": "prod", "team": "infra"}, metadata.FreeformTags)
}

func TestGetOCIMetadataSendsV2AuthorizationHeader(t *testing.T) {
	t.Parallel()

	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:varnamelen
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")

		responseJSON := `{"id":"test-id","shape":"VM.Standard2.1","canonicalRegionName":"us-ashburn-1",` +
			`"compartmentId":"ocid1.compartment","availabilityDomain":"AD-1","image":"ocid1.image","displayName":"test"}`
		_, _ = w.Write([]byte(responseJSON))
	}))

	defer server.Close()

	origEndpoint := ociEndpoint
	ociEndpoint = server.URL + "/"

	defer func() { ociEndpoint = origEndpoint }()

	_, err := GetOCIMetadata(false)
	require.NoError(t, err)
	require.Equal(t, ociV2AuthorizationHeader, receivedAuth)
}

// output generated with: curl -s -H "Authorization: Bearer Oracle" http://169.254.169.254/opc/v2/vnics/
func TestGetOCIVnicsMetadata(t *testing.T) {
	t.Parallel()

	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:varnamelen
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")

		responseJSON := `[{
			"vnicId": "ocid1.vnic.oc1.phx.abc",
			"privateIp": "10.0.0.5",
			"vlanTag": 0,
			"macAddr": "00:00:17:02:2D:41",
			"virtualRouterIp": "10.0.0.1",
			"subnetCidrBlock": "10.0.0.0/24",
			"nicIndex": 0
		}]`
		_, _ = w.Write([]byte(responseJSON))
	}))

	defer server.Close()

	origEndpoint := ociVnicsEndpoint
	ociVnicsEndpoint = server.URL + "/"

	defer func() { ociVnicsEndpoint = origEndpoint }()

	vnics, err := GetOCIVnicsMetadata(false)
	require.NoError(t, err)
	require.Equal(t, ociV2AuthorizationHeader, receivedAuth)
	require.Len(t, vnics, 1)
	require.Equal(t, "ocid1.vnic.oc1.phx.abc", vnics[0].VnicID)
	require.Equal(t, "10.0.0.5", vnics[0].PrivateIP)
}
