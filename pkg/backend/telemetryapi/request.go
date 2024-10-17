// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi/internal"
)

var (
	maxCompressedSizeBytes                                       = 1_000_000         //nolint:gochecknoglobals
	compressFunc           func(b []byte) (*bytes.Buffer, error) = internal.Compress //nolint:gochecknoglobals
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

var errUnableToSplit = fmt.Errorf("unable to split large payload further")

type config struct {
	data        []metricBatch
	apiKey      string
	url         string
	userAgent   string
	maxEntities int
}

func newBatchRequest(ctx context.Context, r config) (reqs []request, err error) {
	// todo: split payload based on payload size
	if len(r.data) < 1 {
		return nil, nil
	}

	if len(r.data) <= r.maxEntities {
		return buildRequests(ctx, r.data, r.apiKey, r.url, r.userAgent)
	}

	metrics := r.data[:r.maxEntities]
	req, err := buildRequests(ctx, metrics, r.apiKey, r.url, r.userAgent)
	reqs = append(reqs, req...)

	if len(r.data[r.maxEntities:]) > 0 {
		r.data = r.data[r.maxEntities:]
		req, err = newBatchRequest(ctx, r)
		if nil != err {
			return nil, err
		}
		reqs = append(reqs, req...)
	}

	return reqs, err
}

// buildRequestsForSplittedMetricBatches will spit metricsBatch in two and build requests for each of them.
// If there is only one batch, it will be splitted in two and call itself to create the requests.
func buildRequestsForSplittedMetricBatches(ctx context.Context, metricsBatch []metricBatch, apiKey string, url string, userAgent string) ([]request, error) {
	if len(metricsBatch) == 0 {
		return nil, nil
	}

	if len(metricsBatch) > 1 {
		half := len(metricsBatch) / 2 //nolint:gomnd

		reqs1, err := buildRequests(ctx, metricsBatch[:half], apiKey, url, userAgent)
		if err != nil {
			return nil, err
		}

		reqs2, err := buildRequests(ctx, metricsBatch[half:], apiKey, url, userAgent)
		if err != nil {
			return nil, err
		}

		return append(reqs1, reqs2...), nil
	}

	return buildRequestsForSplittedMetricBatches(ctx, metricsBatch[0].splitBatch(), apiKey, url, userAgent)
}

func buildRequests(ctx context.Context, metricsBatch []metricBatch, apiKey string, url string, userAgent string) ([]request, error) {
	var entityIds string
	buf := &bytes.Buffer{}
	buf.WriteByte('[')

	for i := range metricsBatch {
		metricsBatch[i].writeSingleJSON(buf)
		// add ',' separator between metrics
		if i < len(metricsBatch)-1 {
			buf.WriteByte(',')
		}

		// collect unique entityID from metric
		if strings.Contains(entityIds, metricsBatch[i].Identity) {
			continue
		}
		if i > 0 {
			entityIds = entityIds + ","
		}
		entityIds = entityIds + metricsBatch[i].Identity
	}

	buf.WriteByte(']')

	uncompressed := buf.Bytes()

	compressed, err := compressFunc(uncompressed)
	if nil != err {
		return nil, fmt.Errorf("error compressing data: %w", err)
	}

	// If compressed payload size is bigger than limit, split the metricBatches and build requests for them.
	// If a single metricBatch is bigger than limit, then the it will be splitted in two, splitting the
	// metrics
	compressedLen := compressed.Len()
	if compressedLen > maxCompressedSizeBytes {
		return buildRequestsForSplittedMetricBatches(ctx, metricsBatch, apiKey, url, userAgent)
	}

	req, err := createRequest(ctx, uncompressed, compressed, compressedLen, apiKey, url, userAgent)
	if err != nil {
		return nil, err
	}

	logger.WithTraceField("json", string(uncompressed)).Debug("Request created")
	req.Request.Header.Add("X-NRI-Entity-Ids", entityIds)
	return []request{req}, err
}

func requestNeedsSplit(r request) bool {
	return r.compressedBodyLength >= maxCompressedSizeBytes
}

func newRequests(ctx context.Context, batch requestsBuilder, apiKey string, url string, userAgent string) ([]request, error) {
	return newRequestsInternal(ctx, batch, apiKey, url, userAgent, requestNeedsSplit)
}

//nolint:nonamedreturns
func createRequest(ctx context.Context, rawJSON json.RawMessage, compressed *bytes.Buffer, compressedLen int, apiKey string, url string, userAgent string) (req request, err error) {
	reqHTTP, err := http.NewRequest("POST", url, compressed)
	if nil != err {
		return req, fmt.Errorf("error creating request: %v", err)
	}

	logger.WithTraceField("compressed_data_size", compressedLen).Debug("Request created")

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
	// control size
	uncompressed := batch.makeBody()

	compressed, err := compressFunc(uncompressed)
	if nil != err {
		return nil, fmt.Errorf("error compressing data: %w", err)
	}
	compressedLen := compressed.Len()

	req, err := createRequest(ctx, uncompressed, compressed, compressedLen, apiKey, url, userAgent)
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
