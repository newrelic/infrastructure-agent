// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package identityapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
	"time"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
)

var (
	testRegisterRequestPath    = "identity/v1/register/batch"
	testRegisterEntity         = []RegisterEntity{NewRegisterEntity("my-entity-1"), NewRegisterEntity("my-entity-2")}
	testRegisterEntityResponse = []RegisterEntityResponse{{entity.ID(12345), "my-entity-1"}, {entity.ID(54321), "my-entity-2"}}
)

func getRegisterRequestBody(req *http.Request) ([]RegisterEntity, error) {
	gzipReader, err := gzip.NewReader(req.Body)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	buf, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}

	var body []RegisterEntity
	err = json.Unmarshal(buf, &body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func getRegisterResponse(entities []RegisterEntityResponse) (*http.Response, error) {
	buf, err := json.Marshal(entities)
	if err != nil {
		return nil, err
	}
	reader := ioutil.NopCloser(bytes.NewReader(buf))
	response := &http.Response{Status: "200 OK", StatusCode: 200, Body: reader, Header: http.Header{}}
	return response, nil
}

func TestRegisterRetryTime(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		reqBody, err := getRegisterRequestBody(req)
		assert.NoError(t, err)
		assert.EqualValues(t, testRegisterEntity, reqBody)

		var entities []RegisterEntityResponse
		resp, err := getRegisterResponse(entities)
		assert.NoError(t, err)
		resp.Header.Add("Retry-After", "10")
		resp.StatusCode = 429
		resp.Status = "429 TOO MANY REQUESTS"

		return resp, nil
	}

	client, err := NewIdentityRegisterClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, mockHttp)
	assert.NoError(t, err)

	entities, retryTime, err := client.Register(testAgentEntityId, testRegisterEntity)
	assert.Error(t, err)
	assert.EqualValues(t, time.Duration(10*time.Second), retryTime)

	var expected []RegisterEntityResponse
	assert.EqualValues(t, expected, entities)
}

func TestRegisterOk(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		return getRegisterResponse(testRegisterEntityResponse)
	}

	client, err := NewIdentityRegisterClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, mockHttp)
	assert.NoError(t, err)

	entities, retryTime, err := client.Register(testAgentEntityId, testRegisterEntity)
	assert.NoError(t, err)
	assert.EqualValues(t, EmptyRetryTime, retryTime)
	assert.EqualValues(t, testRegisterEntityResponse, entities)
}

func TestRegisterMakeUrl(t *testing.T) {
	client := registerClient{svcUrl: testUrl}

	path := client.makeURL(testRegisterRequestPath)
	assert.EqualValues(t, testUrl+"/"+testRegisterRequestPath, path)
}

func TestRegisterRequestHeaders(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		return nil, nil
	}
	client := registerClient{userAgent: testUserAgent, licenseKey: testLicenseKey, httpClient: mockHttp}

	request, err := http.NewRequest("POST", testUrl, nil)
	assert.NoError(t, err)

	response, err := client.do(request, testAgentEntityId)
	assert.NoError(t, err)
	assert.Nil(t, response)
	assert.EqualValues(t, "application/json", request.Header.Get("Content-Type"))
	assert.EqualValues(t, testUserAgent, request.Header.Get("User-Agent"))
	assert.EqualValues(t, testLicenseKey, request.Header.Get(backendhttp.LicenseHeader))

	agentID, err := strconv.ParseInt(request.Header.Get(backendhttp.AgentEntityIdHeader), 10, 64)
	assert.NoError(t, err)
	assert.EqualValues(t, testAgentEntityId, agentID)
}

func TestRegisterMarshallNoCompression(t *testing.T) {
	client := &registerClient{compressionLevel: gzip.NoCompression}

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

func TestRegisterMarshallCompression(t *testing.T) {
	client := &registerClient{compressionLevel: gzip.BestCompression}

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
