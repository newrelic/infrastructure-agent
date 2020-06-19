// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package identityapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
)

var (
	testConnectPath   = "identity/v1/connect"
	testUrl           = "https://staging.newrelic.com"
	testLicenseKey    = "1234567890"
	testUserAgent     = "Agent v1.2.3"
	testAgentEntityId = entity.ID(999666333)
	testAgentGUID     = "FOO"
	expectedGUID      = entity.GUID(testAgentGUID)
	expectedIdentity  = entity.Identity{ID: testAgentEntityId, GUID: expectedGUID}
)

func generateDefaultFingerprint() fingerprint.Fingerprint {
	return fingerprint.Fingerprint{
		FullHostname:    "test1.newrelic.com",
		Hostname:        "test1",
		CloudProviderId: "1234abc",
		DisplayName:     "foobar",
		BootID:          "qwerty1234",
		IpAddresses:     map[string][]string{},
		MacAddresses:    map[string][]string{},
	}
}

func getConnectBody(req *http.Request) (*postConnectBody, error) {
	gzipReader, err := gzip.NewReader(req.Body)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	buf, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}

	body := &postConnectBody{}
	err = json.Unmarshal(buf, body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func getConnectResponse() (*http.Response, error) {
	return _getConnectResponse(true)
}

func _getConnectResponse(withGUID bool) (*http.Response, error) {
	body := &postConnectResponse{
		Identity: IdentityResponse{
			EntityId: testAgentEntityId,
		},
	}
	if withGUID {
		body.Identity.GUID = testAgentGUID
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	reader := ioutil.NopCloser(bytes.NewReader(buf))
	response := &http.Response{Status: "200 OK", StatusCode: 200, Body: reader, Header: http.Header{}}
	return response, nil
}

func TestConnectRetryTime(t *testing.T) {
	fp := generateDefaultFingerprint()

	mockHttp := func(req *http.Request) (*http.Response, error) {
		reqBody, err := getConnectBody(req)
		assert.NoError(t, err)

		assert.EqualValues(t, "container", reqBody.Type)
		assert.EqualValues(t, "v1", reqBody.Protocol)
		assert.EqualValues(t, fp, reqBody.Fingerprint)

		resp, err := getConnectResponse()
		assert.NoError(t, err)
		resp.Header.Add("Retry-After", "10")
		resp.StatusCode = 429
		resp.Status = "429 TOO MANY REQUESTS"

		return resp, nil
	}

	client, err := NewIdentityConnectClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, true, mockHttp)
	assert.NoError(t, err)

	entityId, retryPolicy, err := client.Connect(fp)
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), "ingest rejected connect: 429 429 TOO MANY REQUESTS {\"identity\":{\"entityId\":999666333,\"GUID\":\"FOO\"}}")
	}

	expectedRetryPolicy := backendhttp.RetryPolicy{
		After:      10 * time.Second,
		MaxBackOff: 5 * time.Minute,
	}
	assert.EqualValues(t, expectedRetryPolicy, retryPolicy)
	assert.EqualValues(t, entity.EmptyIdentity, entityId)
}

func TestConnectErrorNoRetryAfterHeader(t *testing.T) {
	fp := generateDefaultFingerprint()

	mockHttp := func(req *http.Request) (*http.Response, error) {
		reqBody, err := getConnectBody(req)
		assert.NoError(t, err)

		assert.EqualValues(t, "container", reqBody.Type)
		assert.EqualValues(t, "v1", reqBody.Protocol)
		assert.EqualValues(t, fp, reqBody.Fingerprint)

		resp, err := getConnectResponse()
		assert.NoError(t, err)
		resp.StatusCode = 500
		resp.Status = "500 SERVER ERROR"

		return resp, nil
	}

	client, err := NewIdentityConnectClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, true, mockHttp)
	assert.NoError(t, err)

	entityId, retryPolicy, err := client.Connect(fp)
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), "ingest rejected connect: 500 500 SERVER ERROR {\"identity\":{\"entityId\":999666333,\"GUID\":\"FOO\"}}")
	}

	expectedRetryPolicy := backendhttp.RetryPolicy{
		After:      time.Duration(0),
		MaxBackOff: 5 * time.Minute,
	}
	assert.EqualValues(t, expectedRetryPolicy, retryPolicy)
	assert.EqualValues(t, entity.EmptyIdentity, entityId)
}

func TestConnectErrorTrialExpired(t *testing.T) {
	fp := generateDefaultFingerprint()

	mockHttp := func(req *http.Request) (*http.Response, error) {
		reqBody, err := getConnectBody(req)
		assert.NoError(t, err)

		assert.EqualValues(t, "container", reqBody.Type)
		assert.EqualValues(t, "v1", reqBody.Protocol)
		assert.EqualValues(t, fp, reqBody.Fingerprint)

		resp, err := getConnectResponse()
		assert.NoError(t, err)
		resp.StatusCode = http.StatusForbidden
		resp.Status = "403 forbidden"

		return resp, nil
	}

	client, err := NewIdentityConnectClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, true, mockHttp)
	assert.NoError(t, err)

	entityId, retryPolicy, err := client.Connect(fp)
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), "ingest rejected connect: 403 403 forbidden {\"identity\":{\"entityId\":999666333,\"GUID\":\"FOO\"}}")
	}

	expectedRetryPolicy := backendhttp.RetryPolicy{
		After:      time.Duration(0),
		MaxBackOff: 1 * time.Hour,
	}
	assert.EqualValues(t, expectedRetryPolicy, retryPolicy)
	assert.EqualValues(t, entity.EmptyIdentity, entityId)
}

func TestConnectErrorTrialInactive(t *testing.T) {
	fp := generateDefaultFingerprint()

	mockHttp := func(req *http.Request) (*http.Response, error) {
		reqBody, err := getConnectBody(req)
		assert.NoError(t, err)

		assert.EqualValues(t, "container", reqBody.Type)
		assert.EqualValues(t, "v1", reqBody.Protocol)
		assert.EqualValues(t, fp, reqBody.Fingerprint)

		resp, err := getConnectResponse()
		assert.NoError(t, err)
		resp.StatusCode = http.StatusForbidden
		resp.Status = "403 forbidden"
		resp.Header.Add(backendhttp.TrialStatusHeader, "starting")

		return resp, nil
	}

	client, err := NewIdentityConnectClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, true, mockHttp)
	assert.NoError(t, err)

	entityId, retryPolicy, err := client.Connect(fp)
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), "ingest rejected connect: 403 403 forbidden {\"identity\":{\"entityId\":999666333,\"GUID\":\"FOO\"}}")
	}

	expectedRetryPolicy := backendhttp.RetryPolicy{
		After:      time.Duration(0),
		MaxBackOff: 5 * time.Minute,
	}
	assert.EqualValues(t, expectedRetryPolicy, retryPolicy)
	assert.EqualValues(t, entity.EmptyIdentity, entityId)
}

func TestConnectFingerprint(t *testing.T) {
	fp := generateDefaultFingerprint()

	mockHttp := func(req *http.Request) (*http.Response, error) {
		reqBody, err := getConnectBody(req)
		assert.NoError(t, err)

		assert.EqualValues(t, "container", reqBody.Type)
		assert.EqualValues(t, "v1", reqBody.Protocol)
		assert.EqualValues(t, fp, reqBody.Fingerprint)

		return getConnectResponse()
	}

	client, err := NewIdentityConnectClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, true, mockHttp)
	assert.NoError(t, err)

	entityId, retryPolicy, err := client.Connect(fp)
	assert.NoError(t, err)

	expectedRetryPolicy := backendhttp.RetryPolicy{
		After:      time.Duration(0),
		MaxBackOff: time.Duration(0),
	}

	assert.EqualValues(t, expectedRetryPolicy, retryPolicy)
	assert.EqualValues(t, expectedIdentity, entityId)
}

func TestConnectOk(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		return getConnectResponse()
	}

	client, err := NewIdentityConnectClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, true, mockHttp)
	assert.NoError(t, err)

	entityId, retryPolicy, err := client.Connect(fingerprint.Fingerprint{})
	assert.NoError(t, err)
	assert.EqualValues(t, EmptyRetryTime, retryPolicy.After)
	assert.EqualValues(t, EmptyRetryTime, retryPolicy.MaxBackOff)

	assert.EqualValues(t, expectedIdentity, entityId)
}

func TestConnectOkWithNoGUID(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		return _getConnectResponse(false)
	}

	client, err := NewIdentityConnectClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, true, mockHttp)
	assert.NoError(t, err)

	entityId, retryPolicy, err := client.Connect(fingerprint.Fingerprint{})
	assert.NoError(t, err)
	assert.EqualValues(t, EmptyRetryTime, retryPolicy.After)
	assert.EqualValues(t, EmptyRetryTime, retryPolicy.MaxBackOff)

	assert.EqualValues(t, entity.Identity{ID: testAgentEntityId}, entityId)
}

func TestConnectMakeUrl(t *testing.T) {
	client := identityClient{svcUrl: testUrl}

	path := client.makeURL(testConnectPath)
	assert.EqualValues(t, testUrl+"/"+testConnectPath, path)
}

func TestConnectRequestHeaders(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		return nil, nil
	}
	client := identityClient{userAgent: testUserAgent, licenseKey: testLicenseKey, httpClient: mockHttp}

	request, err := http.NewRequest("POST", testUrl, nil)
	assert.NoError(t, err)

	response, err := client.do(request)
	assert.NoError(t, err)
	assert.Nil(t, response)
	assert.EqualValues(t, "application/json", request.Header.Get("Content-Type"))
	assert.EqualValues(t, testUserAgent, request.Header.Get("User-Agent"))
	assert.EqualValues(t, testLicenseKey, request.Header.Get(backendhttp.LicenseHeader))
}

func TestConnectMarshallNoCompression(t *testing.T) {
	client := &identityClient{compressionLevel: gzip.NoCompression}

	type marshallTest struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	mtInput := marshallTest{Foo: "test1", Bar: 12345}

	buf, err := client.marshal(&mtInput)
	assert.NoError(t, err)

	mtOutput := marshallTest{}
	err = json.Unmarshal(buf.Bytes(), &mtOutput)
	assert.NoError(t, err)
	assert.EqualValues(t, mtInput, mtOutput)
}

func TestConnectMarshallCompression(t *testing.T) {
	client := &identityClient{compressionLevel: gzip.BestCompression}

	type marshallTest struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	mtInput := marshallTest{Foo: "test1", Bar: 12345}

	buf, err := client.marshal(&mtInput)
	assert.NoError(t, err)

	reader := bytes.NewReader(buf.Bytes())
	assert.NotNil(t, reader)

	gzipReader, err := gzip.NewReader(reader)
	assert.NoError(t, err)
	defer gzipReader.Close()

	plainBuf, err := ioutil.ReadAll(gzipReader)
	assert.NoError(t, err)

	mtOutput := marshallTest{}
	err = json.Unmarshal(plainBuf, &mtOutput)
	assert.NoError(t, err)
	assert.EqualValues(t, mtInput, mtOutput)
}

func Test_identityClient_ConnectUpdateReturnsEntityID(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		resp, err := getConnectResponse()
		assert.NoError(t, err)
		resp.Header.Add("Retry-After", "10")
		return resp, nil
	}

	client := identityClient{
		userAgent:  testUserAgent,
		licenseKey: testLicenseKey,
		httpClient: mockHttp,
	}
	fp := generateDefaultFingerprint()

	_, entityID, err := client.ConnectUpdate(entity.Identity{ID: 1}, fp)
	assert.NoError(t, err)
	assert.Equal(t, expectedIdentity, entityID)
}

func Test_identityClient_DisconnectReturnsNoErr(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		resp, err := getConnectResponse()
		assert.NoError(t, err)
		resp.Header.Add("Retry-After", "10")
		return resp, nil
	}

	client := identityClient{
		userAgent:  testUserAgent,
		licenseKey: testLicenseKey,
		httpClient: mockHttp,
	}

	err := client.Disconnect(1, ReasonHostShutdown)

	assert.NoError(t, err)
}
