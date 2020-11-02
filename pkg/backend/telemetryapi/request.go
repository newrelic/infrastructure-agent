// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi/internal"
)

const (
	maxCompressedSizeBytes = 1 << 20
)

// request contains an http.Request and the UncompressedBody which is provided
// for logging.
type request struct {
	Request          *http.Request
	UncompressedBody json.RawMessage

	compressedBody       []byte
	compressedBodyLength int
}

type requestsBuilder interface {
	makeBody() json.RawMessage
	split() []requestsBuilder
}

var (
	errUnableToSplit     = fmt.Errorf("unable to split large payload further")
	maxEntitiesByRequest = 100
)

func newBatchRequest(ctx context.Context, metricsBatch []metricBatch, apiKey string, url string, userAgent string) (reqs []request, err error) {
	// todo: split payload based on payload size
	if len(metricsBatch) < 1 {
		return nil, nil
	}

	if len(metricsBatch) <= maxEntitiesByRequest {
		return buildRequests(ctx, metricsBatch, apiKey, url, userAgent)
	}

	metrics := metricsBatch[:min(len(metricsBatch), maxEntitiesByRequest)]
	req, err := buildRequests(ctx, metrics, apiKey, url, userAgent)
	reqs = append(reqs, req...)

	if len(metricsBatch[maxEntitiesByRequest:]) > 0 {
		req, err = newBatchRequest(ctx, metricsBatch[maxEntitiesByRequest:], apiKey, url, userAgent)
		if nil != err {
			return nil, err
		}
		reqs = append(reqs, req...)
	}

	return reqs, err
}

func buildRequests(ctx context.Context, metricsBatch []metricBatch, apiKey string, url string, userAgent string) ([]request, error) {
	var entityIds string
	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	for i := range metricsBatch {
		metricsBatch[i].writeSingleJSON(buf)
		entityIds = entityIds + metricsBatch[i].Identity
		if i < len(metricsBatch)-1 {
			buf.WriteByte(',')
			entityIds = entityIds + ","
		}
	}
	buf.WriteByte(']')
	req, err := createRequest(ctx, buf.Bytes(), apiKey, url, userAgent)
	if err != nil {
		return nil, err
	}

	jsonPayload := string(buf.Bytes())
	logger.WithField("json", jsonPayload).Debug("Request created")
	req.Request.Header.Add("X-NRI-Entity-Ids", entityIds)
	return []request{req}, err
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func requestNeedsSplit(r request) bool {
	return r.compressedBodyLength >= maxCompressedSizeBytes
}

func newRequests(ctx context.Context, batch requestsBuilder, apiKey string, url string, userAgent string) ([]request, error) {
	return newRequestsInternal(ctx, batch, apiKey, url, userAgent, requestNeedsSplit)
}

func createRequest(ctx context.Context, rawJSON json.RawMessage, apiKey string, url string, userAgent string) (req request, err error) {
	compressed, err := internal.Compress(rawJSON)
	if nil != err {
		return req, fmt.Errorf("error compressing data: %v", err)
	}
	compressedLen := compressed.Len()

	reqHTTP, err := http.NewRequest("POST", url, compressed)
	if nil != err {
		return req, fmt.Errorf("error creating request: %v", err)
	}
	reqHTTP.Header.Add("Content-Type", "application/json")
	reqHTTP.Header.Add("Api-Key", apiKey)
	reqHTTP.Header.Add("Content-Encoding", "gzip")
	reqHTTP.Header.Add("User-Agent", userAgent)
	req = request{
		Request:              reqHTTP.WithContext(ctx),
		UncompressedBody:     rawJSON,
		compressedBody:       compressed.Bytes(),
		compressedBodyLength: compressedLen,
	}
	return req, err
}

func newRequestsInternal(ctx context.Context, batch requestsBuilder, apiKey string, url string, userAgent string, needsSplit func(request) bool) ([]request, error) {
	req, err := createRequest(ctx, batch.makeBody(), apiKey, url, userAgent)
	if err != nil {
		return nil, err
	}

	if !needsSplit(req) {
		return []request{req}, nil
	}

	var reqs []request
	batches := batch.split()
	if nil == batches {
		return nil, errUnableToSplit
	}

	for _, b := range batches {
		rs, err := newRequestsInternal(ctx, b, apiKey, url, userAgent, needsSplit)
		if nil != err {
			return nil, err
		}
		reqs = append(reqs, rs...)
	}
	return reqs, nil
}
