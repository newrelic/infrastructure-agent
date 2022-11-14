// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
)

// output generated with: curl -s -H Metadata:true --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2017-12-01"
func TestParseAzureMetadata(t *testing.T) {
	response := &http.Response{
		StatusCode: 200,
		Body: ioutil.NopCloser(bytes.NewBuffer([]byte(`{
			"compute": {
				 "location": "northeurope",
				 "name": "tests",
				 "offer": "0001-com-ubuntu-server-focal",
				 "osType": "Linux",
				 "placementGroupId": "",
				 "platformFaultDomain": "0",
				 "platformUpdateDomain": "0",
				 "publisher": "canonical",
				 "resourceGroupName": "testing",
				 "sku": "20_04-lts-gen2",
				 "subscriptionId": "11111-2222-3333-4444-5555",
				 "tags": "env:testing;team:best",
				 "version": "20.04.202210180",
				 "vmId": "aaaaaa-bbbbb-cccc-dddd-aaaaaaa3a749",
				 "vmScaleSetName": "",
				 "vmSize": "Standard_B2s",
				 "zone": "1"
			  },
			  "network": {
				 "interface": [
				   {
					  "ipv4": {
						"ipAddress": [
						  {
							"privateIpAddress": "10.0.0.4",
							"publicIpAddress": ""
						  }
						],
						"subnet": [
						  {
							"address": "10.0.0.0",
							"prefix": "24"
						  }
						]
					  },
					  "ipv6": {
						"ipAddress": []
					  },
					  "macAddress": "00000AAAAAAA"
				   }
				 ]
			  }
	}`))),
	}

	metadata, err := parseAzureMetadataResponse(response)
	assert.NoError(t, err)
	assert.Equal(t, metadata.Compute.SubscriptionID, "11111-2222-3333-4444-5555")
	assert.Equal(t, metadata.Compute.Location, "northeurope")
	assert.Equal(t, metadata.Compute.Zone, "1")
	assert.Equal(t, metadata.Compute.VmSize, "Standard_B2s")
	assert.Equal(t, metadata.Compute.VmId, "aaaaaa-bbbbb-cccc-dddd-aaaaaaa3a749")
}
