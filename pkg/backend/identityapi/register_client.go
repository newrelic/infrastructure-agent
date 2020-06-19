// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package identityapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var rlog = log.WithComponent("IdentityRegisterClient")

type IdentityRegisterClient interface {
	Register(agentEntityID entity.ID, entities []RegisterEntity) ([]RegisterEntityResponse, time.Duration, error)
}

type registerClient struct {
	svcUrl           string
	licenseKey       string
	userAgent        string
	httpClient       backendhttp.Client
	compressionLevel int
}

type RegisterEntity struct {
	Key        entity.Key        `json:"entityKey"`
	Name       string            `json:"entityName,omitempty"`
	Type       string            `json:"entityType,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Interfaces []string          `json:"interfaces,omitempty"`
}

type RegisterEntityResponse struct {
	ID  entity.ID  `json:"entityID"`
	Key entity.Key `json:"entityKey"`
}

func NewRegisterEntity(key entity.Key) RegisterEntity {
	return RegisterEntity{key, "", "", nil, nil}
}

func NewIdentityRegisterClient(
	svcUrl, licenseKey, userAgent string,
	compressionLevel int,
	httpClient backendhttp.Client,
) (IdentityRegisterClient, error) {
	if compressionLevel < gzip.NoCompression || compressionLevel > gzip.BestCompression {
		return nil, fmt.Errorf("gzip: invalid compression level: %d", compressionLevel)
	}
	return &registerClient{
		svcUrl:           strings.TrimSuffix(svcUrl, "/"),
		licenseKey:       licenseKey,
		userAgent:        userAgent,
		httpClient:       httpClient,
		compressionLevel: compressionLevel,
	}, nil
}

// Perform the GetIDs step. For doing that, the Agent must provide for each entity an entityKey. Backend should reply
// with a unique Entity ID across NR for each registered entity
func (rc *registerClient) Register(agentID entity.ID, entities []RegisterEntity) (ids []RegisterEntityResponse, retryAfter time.Duration, err error) {
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
