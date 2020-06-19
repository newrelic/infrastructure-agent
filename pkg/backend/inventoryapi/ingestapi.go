// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package inventoryapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var ilog = log.WithComponent("IngestClient")

// IngestError is an error type that only occurs in the bad status code case.
type IngestError struct {
	msg        string
	Status     string
	StatusCode int
	Body       string
}

func (e *IngestError) Error() string {
	return fmt.Sprintf("InventoryIngest: %s: %d %s %s", e.msg, e.StatusCode, e.Status, e.Body)
}

// NewIngestError returns a new IngestError.
func NewIngestError(msg string, code int, status, body string) *IngestError {
	return &IngestError{msg, status, code, body}
}

type IngestClient struct {
	svcUrl           string
	licenseKey       string
	userAgent        string
	agentKey         string
	agentIDProvide   id.Provide
	connectEnabled   bool
	HttpClient       backendhttp.Client
	CompressionLevel int
}

func NewIngestClient(
	svcUrl, licenseKey, userAgent string,
	compressionLevel int,
	agentKey string,
	agentIDProvide id.Provide,
	connectEnabled bool,
	httpClient backendhttp.Client,
) (*IngestClient, error) {
	if compressionLevel < gzip.NoCompression || compressionLevel > gzip.BestCompression {
		return nil, fmt.Errorf("gzip: invalid compression level: %d", compressionLevel)
	}
	return &IngestClient{
		svcUrl:           strings.TrimSuffix(svcUrl, "/"),
		licenseKey:       licenseKey,
		userAgent:        userAgent,
		agentKey:         agentKey,
		agentIDProvide:   agentIDProvide,
		HttpClient:       httpClient,
		connectEnabled:   connectEnabled,
		CompressionLevel: compressionLevel,
	}, nil
}

func (i *IngestClient) makeURL(requestPath string) string {
	requestPath = strings.TrimPrefix(requestPath, "/")
	return fmt.Sprintf("%s/%s", i.svcUrl, requestPath)
}

// Do performs an http.Request, augmenting it with auth headers
func (i *IngestClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", i.userAgent)
	req.Header.Set(backendhttp.LicenseHeader, i.licenseKey)
	req.Header.Set(backendhttp.EntityKeyHeader, i.agentKey)

	if i.agentKey == "" {
		ilog.Warn("no available agent-key on ingest client")
	}

	if i.connectEnabled {
		agentIdn := i.agentIDProvide()

		// should never happen, nor send data to backend
		if agentIdn.ID.IsEmpty() {
			return nil, fmt.Errorf("empty agent ID on ingest client")
		}

		req.Header.Set(backendhttp.AgentEntityIdHeader, agentIdn.ID.String())
	}

	return i.HttpClient(req)
}

// PostDeltas posts deltas to inventory ingest. The deltas are assumed to all be coming from one
// logical entity (host, container, etc) and blending deltas together will lead to confusion.
func (ic *IngestClient) PostDeltas(entityKeys []string, isAgent bool, deltas ...*RawDelta) (*PostDeltaResponse, error) {
	deltas = filterDeltas(deltas)

	postDeltaBody := PostDeltaBody{
		ExternalKeys: entityKeys,
		IsAgent:      &isAgent,
		Deltas:       deltas,
	}

	if ic.connectEnabled && isAgent {
		postDeltaBody.EntityID = ic.agentIDProvide().ID
	}

	buf, err := ic.marshal(postDeltaBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", ic.makeURL("/deltas"), buf)
	if err != nil {
		return nil, fmt.Errorf("New request failed: %s", err)
	}
	if ic.CompressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := ic.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Unable to submit state changes for entity %v: %s", entityKeys, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to read server response: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return nil, NewIngestError("inventory deltas were not accepted", resp.StatusCode, resp.Status, string(body))
	}

	var res struct {
		Payload *PostDeltaResponse `json:"payload"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("Unable to parse response JSON: %s", err)
	}

	return res.Payload, nil
}

func (ic *IngestClient) marshal(b interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if ic.CompressionLevel > gzip.NoCompression {
		gzipWriter, err := gzip.NewWriterLevel(&buf, ic.CompressionLevel)
		if err != nil {
			return nil, fmt.Errorf("Unable to create gzip writer: %v", err)
		}
		if err := json.NewEncoder(gzipWriter).Encode(b); err != nil {
			return nil, fmt.Errorf("Gzip writer was not able to write to request body: %s", err)
		}
		if err := gzipWriter.Close(); err != nil {
			return nil, fmt.Errorf("Gzip writer did not close: %s", err)
		}
	} else {
		if err := json.NewEncoder(&buf).Encode(b); err != nil {
			return nil, err
		}
	}
	return &buf, nil
}

// PostDeltasBulk allows posting deltas for multiple entities in a single request.
// On an IngestError, all processed deltas will be returned with a non-empty Error
// string for any that errored.
func (ic *IngestClient) PostDeltasBulk(reqs []PostDeltaBody) ([]BulkDeltaResponse, error) {
	for _, req := range reqs {
		req.Deltas = filterDeltas(req.Deltas)

		if ic.connectEnabled && *req.IsAgent {
			req.EntityID = ic.agentIDProvide().ID
		}
	}

	buf, err := ic.marshal(reqs)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", ic.makeURL("/deltas/bulk"), buf)
	if err != nil {
		return nil, fmt.Errorf("New request failed: %s", err)
	}
	if ic.CompressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := ic.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Unable to submit deltas: %s", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to read server response: %s", err)
	}

	var res struct {
		Payload []BulkDeltaResponse `json:"payload"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("Unable to parse response JSON: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return res.Payload, NewIngestError("inventory deltas were not accepted", resp.StatusCode, resp.Status, string(body))
	}

	return res.Payload, nil
}

type PostDeltaVortexBody struct {
	EntityID entity.ID `json:"entity_id"`

	// Is this entity an agent's own host? Controls whether we display the entity as such and
	// track its connected status. Pointer allows nil for older agents which didn't send this field.
	IsAgent *bool `json:"isAgent"`

	Deltas []*RawDelta `json:"deltas"`
}

// PostDeltasVortex posts deltas to inventory ingest. The deltas are assumed to all be coming from one
// logical entity (host, container, etc) and blending deltas together will lead to confusion.
func (ic *IngestClient) PostDeltasVortex(entityID entity.ID, entityKeys []string, isAgent bool, deltas ...*RawDelta) (*PostDeltaResponse, error) {
	deltas = filterDeltas(deltas)

	buf, err := ic.marshal(PostDeltaVortexBody{entityID, &isAgent, deltas})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", ic.makeURL("/deltas"), buf)
	if err != nil {
		return nil, fmt.Errorf("New request failed: %s", err)
	}
	if ic.CompressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := ic.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Unable to submit state changes for entityID: %d entity %v: %s", entityID, entityKeys, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to read server response: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return nil, NewIngestError("inventory deltas were not accepted", resp.StatusCode, resp.Status, string(body))
	}

	var res struct {
		Payload *PostDeltaResponse `json:"payload"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("Unable to parse response JSON: %s", err)
	}

	return res.Payload, nil
}
