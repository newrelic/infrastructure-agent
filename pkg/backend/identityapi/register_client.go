// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package identityapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/antihax/optional"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/identity-client"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var rlog = log.WithComponent("identityapi.RegisterClient")

const (
	identityPath = "/identity/v1"
)

// RegisterClient provides the ability to register either a single entity or a
// "batch" of entities.
type RegisterClient interface {

	// Deprecated: method to be removed at the end of this completing this feature
	RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []RegisterEntityRemoveMe) ([]RegisterEntityResponse, time.Duration, error)

	// RegisterBatchEntities registers a slice of RegisterEntity. This is done as a batch process
	RegisterBatchEntities(agentEntityID entity.ID, entities []RegisterEntity) ([]RegisterEntityResponse, time.Duration, error)

	// RegisterEntityRemoveMe registers a RegisterEntity
	RegisterEntity(agentEntityID entity.ID, entity RegisterEntity) (RegisterEntityResponse, error)
}

type registerClient struct {
	svcUrl           string
	licenseKey       string
	userAgent        string
	httpClient       backendhttp.Client
	compressionLevel int
	apiClient        apiClient
}

type RegisterEntityRemoveMe struct {
	Key        entity.Key        `json:"entityKey"`
	Name       string            `json:"entityName,omitempty"`
	Type       string            `json:"entityType,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Interfaces []string          `json:"interfaces,omitempty"`
}

type RegisterEntity struct {
	EntityName  string            `json:"entityName"`
	DisplayName string            `json:"displayName,omitempty"`
	EntityType  string            `json:"entityType,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Source      string            `json:"source,omitempty"`
}

type RegisterEntityResponse struct {
	ID   entity.ID  `json:"entityID"`
	Key  entity.Key `json:"entityKey"`
	Name string     `json:"entityName"`
	Err  string     `json:"err"`
}

func NewRegisterEntityResponse(id entity.ID, key entity.Key, name string, err string) RegisterEntityResponse {
	return RegisterEntityResponse{
		ID:   id,
		Key:  key,
		Name: name,
		Err:  err,
	}
}

func NewRegisterEntity(key entity.Key) RegisterEntityRemoveMe {
	return RegisterEntityRemoveMe{key, "", "", nil, nil}
}

// NewRegisterClient returns an implementation of RegisterClient
func NewRegisterClient(
	svcUrl, licenseKey, userAgent string,
	compressionLevel int,
	httpClient backendhttp.Client,
) (RegisterClient, error) {
	if compressionLevel < gzip.NoCompression || compressionLevel > gzip.BestCompression {
		return nil, fmt.Errorf("gzip: invalid compression level: %d", compressionLevel)
	}
	icfg := identity.NewConfiguration()
	icfg.BasePath = svcUrl + identityPath
	icfg.Debug = true
	// TODO: add the global HTTP client here
	// icfg.HTTPClient = httpClient
	identityClient := identity.NewAPIClient(icfg)
	return &registerClient{
		svcUrl:           strings.TrimSuffix(svcUrl, "/"),
		licenseKey:       licenseKey,
		userAgent:        userAgent,
		httpClient:       httpClient,
		compressionLevel: compressionLevel,
		apiClient:        identityClient.DefaultApi,
	}, nil
}

func (rc *registerClient) RegisterBatchEntities(agentEntityID entity.ID, entities []RegisterEntity) (resp []RegisterEntityResponse, duration time.Duration, err error) {

	ctx := context.Background()

	registerRequests := make([]identity.RegisterRequest, len(entities))

	for i := range entities {
		registerRequests[i] = newRegisterRequest(entities[i])
	}

	localVarOptionals := &identity.RegisterBatchPostOpts{
		XNRIAgentEntityId: optional.NewInt64(int64(agentEntityID)),
	}

	apiReps, _, err := rc.apiClient.RegisterBatchPost(ctx, rc.userAgent, rc.licenseKey, registerRequests, localVarOptionals)
	if err != nil {
		return resp, time.Second, err // TODO add right duration
	}

	resp = make([]RegisterEntityResponse, len(apiReps))

	for i := range apiReps {
		resp[i] = NewRegisterEntityResponse(
			entity.ID(apiReps[i].EntityId),
			entity.Key(apiReps[i].EntityName),
			apiReps[i].EntityName,
			apiReps[i].Error)
	}

	return resp, time.Second, err
}

func newRegisterRequest(entity RegisterEntity) identity.RegisterRequest {
	registerRequest := identity.RegisterRequest{
		EntityType:  entity.EntityType,
		EntityName:  entity.EntityName,
		DisplayName: entity.DisplayName,
		Metadata:    entity.Metadata,
	}
	return registerRequest
}

func (rc *registerClient) RegisterEntity(agentEntityID entity.ID, registerEntity RegisterEntity) (resp RegisterEntityResponse, err error) {

	ctx := context.Background()
	registerRequest := newRegisterRequest(registerEntity)
	localVarOptionals := &identity.RegisterPostOpts{
		XNRIAgentEntityId: optional.NewInt64(int64(agentEntityID)),
	}

	apiReps, _, err := rc.apiClient.RegisterPost(ctx, rc.userAgent, rc.licenseKey, registerRequest, localVarOptionals)
	if err != nil {
		return resp, err
	}

	resp = NewRegisterEntityResponse(
		entity.ID(apiReps.EntityId),
		entity.Key(apiReps.EntityName),
		apiReps.EntityName,
		"")

	return resp, err
}

// Perform the GetIDs step. For doing that, the Agent must provide for each entity an entityKey. Backend should reply
// with a unique Entity ID across NR for each registered entity
func (rc *registerClient) RegisterEntitiesRemoveMe(agentID entity.ID, entities []RegisterEntityRemoveMe) (ids []RegisterEntityResponse, retryAfter time.Duration, err error) {
	retryAfter = EmptyRetryTime

	if agentID.IsEmpty() {
		err = ErrEmptyAgentID
		return
	}

	buf, err := rc.marshal(entities)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", rc.makeURL("/register/batch"), buf)
	if err != nil {
		err = fmt.Errorf("register request build failed: %s", err)
		return
	}

	if rc.compressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := rc.do(req, agentID)
	if err != nil {
		err = fmt.Errorf("register request failed: %v", err)
		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.WithError(err).Debug("Error closing ingest body response.")
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("cannot read register response: %s", err)
		return
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		retryAfter, err = time.ParseDuration(resp.Header.Get("Retry-After") + "s")
		if err != nil {
			retryAfter = EmptyRetryTime
		}
		err = inventoryapi.NewIngestError("ingest service rejected the register step", resp.StatusCode, resp.Status, string(body))
		return
	}

	if err = json.Unmarshal(body, &ids); err != nil {
		err = fmt.Errorf("unable to parse register response JSON: %s", err)
		return
	}

	return
}

func (rc *registerClient) makeURL(requestPath string) string {
	requestPath = strings.TrimPrefix(requestPath, "/")
	return fmt.Sprintf("%s/%s", rc.svcUrl, requestPath)
}

// Do performs an http.Request, augmenting it with auth headers
func (rc *registerClient) do(req *http.Request, agentEntityID entity.ID) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", rc.userAgent)
	req.Header.Set(backendhttp.LicenseHeader, rc.licenseKey)
	req.Header.Set(backendhttp.AgentEntityIdHeader, strconv.FormatInt(int64(agentEntityID), 10))

	return rc.httpClient(req)
}

func (rc *registerClient) marshal(b interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if rc.compressionLevel > gzip.NoCompression {
		gzipWriter, err := gzip.NewWriterLevel(&buf, rc.compressionLevel)
		if err != nil {
			return nil, fmt.Errorf("unable to create gzip writer: %v", err)
		}
		defer func() {
			if err := gzipWriter.Close(); err != nil {
				log.WithError(err).Debug("Gzip writer did not close.")
			}
		}()
		if err := json.NewEncoder(gzipWriter).Encode(b); err != nil {
			return nil, fmt.Errorf("gzip writer was not able to write to request body: %s", err)
		}
	} else {
		if err := json.NewEncoder(&buf).Encode(b); err != nil {
			return nil, err
		}

	}
	return &buf, nil
}
