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
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var rlog = log.WithComponent("identityapi.RegisterClient")

const (
	identityPath = "/identity/v1"
)

type RegisterClient interface {
	RegisterEntitiesRemoveMe(agentEntityID entity.ID, entities []RegisterEntity) ([]RegisterEntityResponse, time.Duration, error)
	RegisterProtocolEntities(agentEntityID entity.ID, entities []protocol.Entity) (RegisterBatchEntityResponse, time.Duration, error)
	RegisterEntity(agentEntityID entity.ID, entity protocol.Entity) (RegisterEntityResponse, error)
}

type registerClient struct {
	svcUrl           string
	licenseKey       string
	userAgent        string
	httpClient       backendhttp.Client
	compressionLevel int
	apiClient        apiClient
}

type RegisterEntity struct {
	Key        entity.Key        `json:"entityKey"`
	Name       string            `json:"entityName,omitempty"`
	Type       string            `json:"entityType,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Interfaces []string          `json:"interfaces,omitempty"`
}

type RegisterEntityResponse struct {
	ID   entity.ID  `json:"entityID"`
	Key  entity.Key `json:"entityKey"`
	Name string     `json:"entityName"`
}

type RegisterBatchEntityResponse []RegisterEntityResponse

func NewRegisterEntity(key entity.Key) RegisterEntity {
	return RegisterEntity{key, "", "", nil, nil}
}

func NewIdentityRegisterClient(
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

func (rc *registerClient) RegisterProtocolEntities(agentEntityID entity.ID, entities []protocol.Entity) (resp RegisterBatchEntityResponse, duration time.Duration, err error) {

	ctx := context.Background()

	registerRequests := make([]identity.RegisterRequest, len(entities))

	for i := range entities {
		registerRequest := identity.RegisterRequest{
			EntityType:  entities[i].Type,
			EntityName:  entities[i].Name,
			DisplayName: entities[i].DisplayName,
			Metadata:    convertMetadataToMapStringString(entities[i].Metadata),
		}
		registerRequests[i] = registerRequest
	}

	localVarOptionals := &identity.RegisterBatchPostOpts{
		XNRIAgentEntityId: optional.NewInt64(int64(agentEntityID)),
	}

	apiReps, httpResp, err := rc.apiClient.RegisterBatchPost(ctx, rc.userAgent, rc.licenseKey, registerRequests, localVarOptionals)
	if err != nil {
		rlog.
			WithError(err).
			WithField("XNRIAgentEntityId", agentEntityID).
			WithField("status", httpResp.StatusCode).
			WithField("RegisterRequests", registerRequests).
			Error("Something went wrong")
		return resp, time.Second, err // TODO add right duration
	}

	resp = make(RegisterBatchEntityResponse, len(apiReps))

	for i := range apiReps {
		resp[i] = RegisterEntityResponse{
			ID:   entity.ID(apiReps[i].EntityId),
			Key:  entity.Key(apiReps[i].EntityName),
			Name: apiReps[i].EntityName,
		}
	}

	return resp, time.Second, err
}

func (rc *registerClient) RegisterEntity(agentEntityID entity.ID, ent protocol.Entity) (resp RegisterEntityResponse, err error) {

	ctx := context.Background()
	registerRequest := identity.RegisterRequest{
		EntityType:  ent.Type,
		EntityName:  ent.Name,
		DisplayName: ent.DisplayName,
		Metadata:    convertMetadataToMapStringString(ent.Metadata),
	}
	localVarOptionals := &identity.RegisterPostOpts{
		XNRIAgentEntityId: optional.NewInt64(int64(agentEntityID)),
	}

	apiReps, httpResp, err := rc.apiClient.RegisterPost(ctx, rc.userAgent, rc.licenseKey, registerRequest, localVarOptionals)
	if err != nil {
		rlog.
			WithError(err).
			WithField("XNRIAgentEntityId", agentEntityID).
			WithField("status", httpResp.StatusCode).
			WithField("RegisterRequest", registerRequest).
			Error("Something went wrong")
		return resp, err
	}

	resp = RegisterEntityResponse{
		ID:   entity.ID(apiReps.EntityId),
		Key:  entity.Key(apiReps.EntityName),
		Name: apiReps.EntityName,
	}

	return resp, err
}

// Perform the GetIDs step. For doing that, the Agent must provide for each entity an entityKey. Backend should reply
// with a unique Entity ID across NR for each registered entity
func (rc *registerClient) RegisterEntitiesRemoveMe(agentID entity.ID, entities []RegisterEntity) (ids []RegisterEntityResponse, retryAfter time.Duration, err error) {
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

func convertMetadataToMapStringString(from map[string]interface{}) (to map[string]string) {
	to = make(map[string]string, len(from))
	for key, value := range from {
		to[key] = fmt.Sprintf("%v", value)
	}
	return
}
