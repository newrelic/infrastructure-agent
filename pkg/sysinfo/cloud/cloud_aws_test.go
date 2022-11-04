// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/private/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, token string, tokenCounter *int32, expectedTTL string, expectedFieldName string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(tokenCounter, 1)
		assert.Equal(t, expectedTTL, r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds"))
		_, _ = fmt.Fprint(w, token)
	})
	mux.HandleFunc("/latest/meta-data/"+expectedFieldName, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, token, r.Header.Get("X-aws-ec2-metadata-token"))
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-type", "text/plain")
		_, _ = fmt.Fprint(w, "data")
	})
	mux.HandleFunc("/latest/dynamic/instance-identity/document", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, token, r.Header.Get("X-aws-ec2-metadata-token"))
		assert.Equal(t, "GET", r.Method)
		_, _ = fmt.Fprint(w, `{
					"devpayProductCodes" : null,
					"marketplaceProductCodes" : [ "1abc2defghijklm3nopqrs4tu" ],
					"availabilityZone" : "us-west-2b",
					"privateIp" : "10.158.112.84",
					"version" : "2017-09-30",
					"instanceId" : "i-1234567890abcdef0",
					"billingProducts" : null,
					"instanceType" : "t2.micro",
					"accountId" : "123456789012",
					"imageId" : "ami-5fb8c835",
					"pendingTime" : "2016-11-19T16:32:11Z",
					"architecture" : "x86_64",
					"kernelId" : null,
					"ramdiskId" : null,
					"region" : "us-west-2"
				}`)
	})
	return httptest.NewServer(mux)
}

func TestAWSHarvester_GetRegion(t *testing.T) {
	t.Parallel()
	ts := setupDefaultTestServer(t)
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL

	region, err := h.GetRegion()
	assert.NoError(t, err)
	assert.Equal(t, "us-west-2", region)
}

func setupDefaultTestServer(t *testing.T) *httptest.Server {
	var tokenCounter int32
	token := getRandomToken()
	ts := newTestServer(t, token, &tokenCounter, "600", "")
	return ts
}

func TestAWSHarvester_GetHostType(t *testing.T) {
	t.Parallel()
	ts := setupDefaultTestServer(t)
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL

	hostType, err := h.GetHostType()
	assert.NoError(t, err)
	assert.Equal(t, "t2.micro", hostType)
}

func TestAWSHarvester_GetInstanceID(t *testing.T) {
	t.Parallel()
	ts := setupDefaultTestServer(t)
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL

	instanceID, err := h.GetInstanceID()
	assert.NoError(t, err)
	assert.Equal(t, "i-1234567890abcdef0", instanceID)
}

func TestAWSHarvester_GetAccountID(t *testing.T) {
	t.Parallel()
	ts := setupDefaultTestServer(t)
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL

	instanceID, err := h.GetAccountID()
	assert.NoError(t, err)
	assert.Equal(t, "123456789012", instanceID)
}

func TestAWSHarvester_GetZone(t *testing.T) {
	t.Parallel()
	ts := setupDefaultTestServer(t)
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL

	instanceID, err := h.GetZone()
	assert.NoError(t, err)
	assert.Equal(t, "us-west-2b", instanceID)
}

func TestAWSHarvester_GetInstanceImageID(t *testing.T) {
	t.Parallel()
	ts := setupDefaultTestServer(t)
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL

	instanceID, err := h.GetInstanceImageID()
	assert.NoError(t, err)
	assert.Equal(t, "ami-5fb8c835", instanceID)
}

func TestAWSHarvester_cache(t *testing.T) {
	t.Parallel()
	var tokenCounter int32
	token := getRandomToken()
	ts := newTestServer(t, token, &tokenCounter, "600", "")
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL

	for i := 0; i < 10; i++ {
		assertGetInstanceID(t, h)
	}
	actualCount := int(tokenCounter)
	t.Logf("Got a count of %d", actualCount)
	assert.Equal(t, 1, actualCount, "Got a count of %d rather than 1", actualCount)
}

func TestAWSHarvester_cache_disabled(t *testing.T) {
	t.Parallel()
	var tokenCounter int32
	token := getRandomToken()

	// expected token TTL should be 10 seconds as 0 will cause it to always be invalid
	ts := newTestServer(t, token, &tokenCounter, "10", "")
	defer ts.Close()

	h := NewAWSHarvester(true)
	h.timeout = NewTimeout(-1)
	h.tokenTimeout = NewTimeout(-1)
	h.awsEC2MetadataHostname = ts.URL

	for i := 0; i < 10; i++ {
		assertGetInstanceID(t, h)
	}
	actualCount := int(tokenCounter)
	t.Logf("Got a count of %d", actualCount)
	assert.Equal(t, 10, actualCount, "Got a count of %d rather than 10", actualCount)
}

func TestAWSHarvester_GetAWSMetadataValue(t *testing.T) {
	t.Parallel()
	var tokenCounter int32
	token := getRandomToken()
	fieldName := getRandomToken()
	ts := newTestServer(t, token, &tokenCounter, "600", fieldName)
	defer ts.Close()
	h := NewAWSHarvester(true)
	h.awsEC2MetadataHostname = ts.URL
	data, err := h.GetAWSMetadataValue(fieldName, true)
	require.NoError(t, err)
	assert.Equal(t, "data", data)
}

func TestNewAWSHarvester_expect_correct_const(t *testing.T) {
	require.Equal(t, "http://169.254.169.254", awsEC2MetadataHostname)
}

func getRandomToken() string {
	token := make([]byte, 16)
	rand.Read(token)
	uuid := protocol.UUIDVersion4(token)
	return uuid
}

func assertGetInstanceID(t *testing.T, h *AWSHarvester) {
	time.Sleep(time.Millisecond * 10) // Sleep for 10 Milliseconds as Windows time not accurate
	instanceID, err := h.GetInstanceID()
	assert.NoError(t, err)
	assert.Equal(t, "i-1234567890abcdef0", instanceID)
}
