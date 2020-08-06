// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package identityapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/antihax/optional"
	"github.com/newrelic/infrastructure-agent/pkg/identity-client"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
)

var (
	testRegisterRequestPath    = "identity/v1/register/batch"
	testRegisterEntity         = []RegisterEntity{NewRegisterEntity("my-entity-1"), NewRegisterEntity("my-entity-2")}
	testRegisterEntityResponse = []RegisterEntityResponse{{ID: entity.ID(12345), Name: "my-entity-1"}, {ID: entity.ID(54321), Name: "my-entity-2"}}
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

	client, err := NewRegisterClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, mockHttp)
	assert.NoError(t, err)

	entities, retryTime, err := client.RegisterEntitiesRemoveMe(testAgentEntityId, testRegisterEntity)
	assert.Error(t, err)
	assert.EqualValues(t, 10*time.Second, retryTime)

	var expected []RegisterEntityResponse
	assert.EqualValues(t, expected, entities)
}

func TestRegisterOk(t *testing.T) {
	mockHttp := func(req *http.Request) (*http.Response, error) {
		return getRegisterResponse(testRegisterEntityResponse)
	}

	client, err := NewRegisterClient(testUrl, testLicenseKey, testUserAgent, gzip.BestCompression, mockHttp)
	assert.NoError(t, err)

	entities, retryTime, err := client.RegisterEntitiesRemoveMe(testAgentEntityId, testRegisterEntity)
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

func TestRegisterClient_RegisterEntity(t *testing.T) {
	mc := &mockAPIClient{}
	entID := rand.Int63n(10000)
	expectedRegisterRequest := identity.RegisterRequest{
		EntityType:  "TEST_TYPE",
		EntityName:  "Entity key",
		DisplayName: "Entity Display Name",
		Metadata: map[string]string{
			"key_one":   "value",
			"key_two":   "12345",
			"key_three": "true",
			"key_four":  "1234.56789",
		},
	}
	irr := identity.RegisterResponse{
		EntityId:   entID,
		EntityName: expectedRegisterRequest.EntityName,
		Guid:       "GUIIDIDID",
	}

	agentID := int64(123123123)

	registerPostOpts := &identity.RegisterPostOpts{
		XNRIAgentEntityId: optional.NewInt64(agentID),
	}

	mc.On("RegisterPost",
		mock.AnythingOfType("*context.emptyCtx"),
		"ExpectedUserAgent",
		"ExpectedXLicenseKey",
		expectedRegisterRequest,
		registerPostOpts,
	).Return(irr, &http.Response{}, nil)

	client := &registerClient{
		apiClient:  mc,
		licenseKey: "ExpectedXLicenseKey",
		userAgent:  "ExpectedUserAgent",
	}

	ent := protocol.Entity{
		Type:        expectedRegisterRequest.EntityType,
		Name:        expectedRegisterRequest.EntityName,
		DisplayName: expectedRegisterRequest.DisplayName,
		Metadata: map[string]interface{}{
			"key_one":   "value",
			"key_two":   12345,
			"key_three": true,
			"key_four":  01234.567890,
		},
	}
	agentEntityID := entity.ID(agentID)

	resp, err := client.RegisterEntity(agentEntityID, ent)
	require.NoError(t, err)

	expectedEntityID := entity.ID(entID)
	assert.Equal(t, expectedEntityID, resp.ID)

	expectedEntityKey := entity.Key(expectedRegisterRequest.EntityName)
	assert.Equal(t, expectedEntityKey, resp.Key)

	mc.AssertExpectations(t)
}

func TestRegisterClient_RegisterEntity_err(t *testing.T) {
	expectedError := errors.New("some random error")
	mc := &mockAPIClient{}
	mc.On("RegisterPost",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(identity.RegisterResponse{}, &http.Response{}, expectedError)

	client := &registerClient{
		apiClient:  mc,
		licenseKey: "ExpectedXLicenseKey",
		userAgent:  "ExpectedUserAgent",
	}

	agentEntityID := entity.ID(12231)
	resp, err := client.RegisterEntity(agentEntityID, protocol.Entity{})
	assert.EqualError(t, err, expectedError.Error())
	assert.Equal(t, RegisterEntityResponse{}, resp)
}

type mockAPIClient struct {
	mock.Mock
}

func (m *mockAPIClient) RegisterPost(
	ctx context.Context,
	userAgent string,
	xLicenseKey string,
	registerRequest identity.RegisterRequest,
	localVarOptionals *identity.RegisterPostOpts,
) (identity.RegisterResponse, *http.Response, error) {

	args := m.Called(ctx, userAgent, xLicenseKey, registerRequest, localVarOptionals)
	return args.Get(0).(identity.RegisterResponse),
		args.Get(1).(*http.Response),
		args.Error(2)
}

func (m *mockAPIClient) RegisterBatchPost(
	ctx context.Context,
	userAgent string,
	xLicenseKey string,
	registerRequest []identity.RegisterRequest,
	localVarOptionals *identity.RegisterBatchPostOpts) ([]identity.RegisterBatchEntityResponse, *http.Response, error) {
	args := m.Called(ctx, userAgent, xLicenseKey, registerRequest, localVarOptionals)
	return args.Get(0).([]identity.RegisterBatchEntityResponse),
		args.Get(1).(*http.Response),
		args.Error(2)
}
