// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package identityapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
)

var (
	EmptyRetryTime = time.Duration(0)

	// Errors
	ErrEmptyAgentID = errors.New("empty agent id")
)

var ilog = log.WithComponent("IdentityConnectClient")

type IdentityConnectClient interface {
	Connect(fingerprint fingerprint.Fingerprint, metadata Metadata) (entity.Identity, backendhttp.RetryPolicy, error)
	ConnectUpdate(entityID entity.Identity, fingerprint fingerprint.Fingerprint, metadata Metadata) (backendhttp.RetryPolicy, entity.Identity, error)
	Disconnect(entityID entity.ID, reason DisconnectReason) error
}

type identityClient struct {
	svcUrl           string
	licenseKey       string
	userAgent        string
	httpClient       backendhttp.Client
	compressionLevel int
	containerized    bool
}

type postConnectBody struct {
	Fingerprint fingerprint.Fingerprint `json:"fingerprint"`
	Metadata    Metadata                `json:"metadata"`
	Type        string                  `json:"type"`
	Protocol    string                  `json:"protocol"`
	EntityID    entity.ID               `json:"entityId,omitempty"`
}

type postConnectResponse struct {
	Identity IdentityResponse `json:"identity"`
}

type IdentityResponse struct {
	EntityId entity.ID `json:"entityId"`
	GUID     string    `json:"GUID"`
}

// ToIdentity converts response into entity identity
func (r *IdentityResponse) ToIdentity() entity.Identity {
	return entity.Identity{
		ID:   r.EntityId,
		GUID: entity.GUID(r.GUID),
	}
}

type putDisconnectBody struct {
	EntityID entity.ID        `json:"entityId"`
	Reason   DisconnectReason `json:"reason"`
}

func NewIdentityConnectClient(
	svcUrl, licenseKey, userAgent string,
	compressionLevel int,
	containerizedAgent bool,
	httpClient backendhttp.Client,
) (IdentityConnectClient, error) {
	if compressionLevel < gzip.NoCompression || compressionLevel > gzip.BestCompression {
		return nil, fmt.Errorf("gzip: invalid compression level: %d", compressionLevel)
	}
	return &identityClient{
		svcUrl:           strings.TrimSuffix(svcUrl, "/"),
		licenseKey:       licenseKey,
		userAgent:        userAgent,
		httpClient:       httpClient,
		compressionLevel: compressionLevel,
		containerized:    containerizedAgent,
	}, nil
}

// Perform the Connect step. The Agent must supply a fingerprint for the host. Backend should reply
// with a unique Entity ID across NR.
//
//nolint:cyclop
func (ic *identityClient) Connect(fingerprint fingerprint.Fingerprint, metadata Metadata) (ids entity.Identity, retry backendhttp.RetryPolicy, err error) {
	buf, err := ic.marshal(postConnectBody{
		Fingerprint: fingerprint,
		Metadata:    metadata,
		Type:        ic.agentType(),
		Protocol:    "v1",
	})
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", ic.makeURL("/connect"), buf)
	if err != nil {
		err = fmt.Errorf("connect request failed: %s", err)
		return
	}
	if ic.compressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := ic.do(req)
	if err != nil {
		err = fmt.Errorf("unable to connect: %s", err)
		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			ilog.WithError(err).Debug("Error closing ingest body response.")
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("unable to read server response: %s", err)
		return
	}

	hasError, cause := backendhttp.IsResponseUnsuccessful(resp)

	if hasError {
		retryAfterH := resp.Header.Get("Retry-After")
		if retryAfterH != "" {
			if retry.After, err = time.ParseDuration(retryAfterH + "s"); err != nil {
				ilog.WithError(err).
					Debug("Error parsing connect Retry-After header, continuing with exponential backoff.")
			}
		}

		retry.MaxBackOff = backoff.GetMaxBackoffByCause(cause)
		err = fmt.Errorf("ingest rejected connect: %d %s %s", resp.StatusCode, resp.Status, string(body))
		return
	}

	response := &postConnectResponse{}
	if err = json.Unmarshal(body, response); err != nil {
		err = fmt.Errorf("unable to parse connect response: %s", err)
		return
	}

	ids = response.Identity.ToIdentity()
	return
}

// ConnectUpdate is used to update the host fingerprint of the entityID to the backend.
// nolint:cyclop
func (ic *identityClient) ConnectUpdate(entityIdn entity.Identity, fingerprint fingerprint.Fingerprint, metadata Metadata) (retry backendhttp.RetryPolicy, ids entity.Identity, err error) {
	// Log entry point of the function
	ilog.WithField("entityID", entityIdn.ID).Debug("Attempting to update fingerprint.")

	postBody := postConnectBody{
		Fingerprint: fingerprint,
		Metadata:    metadata, // Use the parameter directly as it now has the correct type
		Type:        ic.agentType(),
		Protocol:    "v1",
		EntityID:    entityIdn.ID,
	}

	// Log the request body before marshalling
	requestBodyBytes, _ := json.Marshal(postBody)
	ilog.WithField("body", string(requestBodyBytes)).Debug("Update request body prepared.")

	buf, err := ic.marshal(postBody)
	if err != nil {
		return
	}

	connectURL := ic.makeURL("/connect")
	// Log the endpoint URL and method
	ilog.WithFields(map[string]interface{}{
		"endpoint": connectURL,
		"method":   http.MethodPut,
	}).Debug("Creating connect update request.")

	req, err := http.NewRequest(http.MethodPut, connectURL, buf)
	if err != nil {
		err = fmt.Errorf("update fingerprint request failed, error: %s", err)
		return
	}
	if ic.compressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// Log the request headers
	headersForLog := make(map[string][]string)
	for k, v := range req.Header {
		headersForLog[k] = v
	}
	ilog.WithField("headers", headersForLog).Debug("Update request headers prepared.")

	resp, err := ic.do(req)
	if err != nil {
		err = fmt.Errorf("unable to update the fingerprint, error: %v", err)
		// Log the error on connection failure
		ilog.WithError(err).Error("Failed to execute connect update request.")
		return
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			ilog.WithError(err).Debug("Error closing ingest body response.")
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("unable to read server response during the fingerprint update, error: %s", err)
		// Log the error on reading response body
		ilog.WithError(err).Error("Failed to read response body for update request.")
		return
	}

	// Log the response status code and the raw response body
	ilog.WithFields(map[string]interface{}{
		"statusCode":   resp.StatusCode,
		"responseBody": string(body),
	}).Debug("Received response from connect update endpoint.")

	hasError, cause := backendhttp.IsResponseUnsuccessful(resp)

	if hasError {
		retryAfterH := resp.Header.Get("Retry-After")
		if retryAfterH != "" {
			if retry.After, err = time.ParseDuration(retryAfterH + "s"); err != nil {
				ilog.WithError(err).
					Debug("Error parsing connect Retry-After header, continuing with exponential backoff.")
			}
		}

		retry.MaxBackOff = backoff.GetMaxBackoffByCause(cause)

		err = inventoryapi.NewIngestError("ingest service rejected the connect step", resp.StatusCode, resp.Status, string(body))
		// Log the rejection details
		ilog.WithError(err).Warn("Connect update request rejected by ingest.")
		return
	}

	pcr := &postConnectResponse{}
	if err = json.Unmarshal(body, pcr); err != nil {
		err = fmt.Errorf("unable to decode connect service response body: %s", err)
		// Log the error on parsing the response
		ilog.WithError(err).Error("Failed to unmarshal connect update response.")
		return
	}

	ids = pcr.Identity.ToIdentity()

	// Log successful update
	ilog.WithField("entityID", ids.ID).Debug("Connect update request successful.")
	return
}

// DisconnectReason is sent with disconnect call.
type DisconnectReason string

const (
	// ReasonHostShutdown is reported when the host running the agent will shutdown.
	ReasonHostShutdown DisconnectReason = "shutdown"
)

// Perform the Disconnect step. The agent will provide the cause and the entityID. Backend should reply
// with a unique Entity ID across NR.
func (ic *identityClient) Disconnect(entityID entity.ID, reason DisconnectReason) error {
	buf, err := ic.marshal(putDisconnectBody{
		EntityID: entityID,
		Reason:   reason,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, ic.makeURL("/disconnect"), buf)
	if err != nil {
		return fmt.Errorf("unable to build disconnect request, error: %v", err)
	}
	if ic.compressionLevel > gzip.NoCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := ic.do(req)
	if err != nil {
		return fmt.Errorf("unable to perform disconnect, error: %s", err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			ilog.WithError(err).Debug("Error closing disconnect body response.")
		}
	}()

	ilog.Debug("Disconnect request performed.")

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read server response for disconnect: %s", err)
	}

	// If not status code 2xx.
	if resp.StatusCode/100 != 2 {
		return inventoryapi.NewIngestError("disconnect request not successful", resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

// agentType returns the type of the agent.
func (ic *identityClient) agentType() string {
	if ic.containerized {
		return "container"
	}
	return "host"
}

func (ic *identityClient) makeURL(requestPath string) string {
	requestPath = strings.TrimPrefix(requestPath, "/")
	return fmt.Sprintf("%s/%s", ic.svcUrl, requestPath)
}

// Do performs an http.Request, augmenting it with auth headers
func (ic *identityClient) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ic.userAgent)
	req.Header.Set(backendhttp.LicenseHeader, ic.licenseKey)

	return ic.httpClient(req)
}

func (ic *identityClient) marshal(b interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if ic.compressionLevel > gzip.NoCompression {
		gzipWriter, err := gzip.NewWriterLevel(&buf, ic.compressionLevel)
		if err != nil {
			return nil, fmt.Errorf("unable to create gzip writer: %v", err)
		}
		defer func() {
			if err := gzipWriter.Close(); err != nil {
				ilog.WithError(err).Debug("Gzip writer did not close.")
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
