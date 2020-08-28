// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
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
	errUnableToSplit = fmt.Errorf("unable to split large payload further")
)

func newBatchRequest(metrics []metricBatch, apiKey string, url string, userAgent string) (reqs []request, err error) {
	// todo: split payload based on:
	// a) number of entities being sent
	// b) payload size
	if len(metrics) < 1 {
		return nil, nil
	}

	var entityIds string
	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	for i := range metrics {
		metrics[i].writeSingleJSON(buf)
		entityIds = entityIds + metrics[i].Identity
		if i < len(metrics)-1 {
			buf.WriteByte(',')
			entityIds = entityIds + ","
		}
	}
	buf.WriteByte(']')
	req, err := createRequest(buf.Bytes(), apiKey, url, userAgent)
	if err != nil {
		return nil, err
	}
	jsonPayload := string(buf.Bytes())
	logger.WithField("json", jsonPayload).Debug("Request created")
	req.Request.Header.Add("X-NRI-Entity-Ids", entityIds)
	reqs = append(reqs, req)
	return reqs, err
}

func requestNeedsSplit(r request) bool {
	return r.compressedBodyLength >= maxCompressedSizeBytes
}

func newRequests(batch requestsBuilder, apiKey string, url string, userAgent string) ([]request, error) {
	return newRequestsInternal(batch, apiKey, url, userAgent, requestNeedsSplit)
}

func createRequest(rawJSON json.RawMessage, apiKey string, url string, userAgent string) (req request, err error) {
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
		Request:              reqHTTP,
		UncompressedBody:     rawJSON,
		compressedBody:       compressed.Bytes(),
		compressedBodyLength: compressedLen,
	}
	return req, err
}

func newRequestsInternal(batch requestsBuilder, apiKey string, url string, userAgent string, needsSplit func(request) bool) ([]request, error) {
	req, err := createRequest(batch.makeBody(), apiKey, url, userAgent)
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
		rs, err := newRequestsInternal(b, apiKey, url, userAgent, needsSplit)
		if nil != err {
			return nil, err
		}
		reqs = append(reqs, rs...)
	}
	return reqs, nil
}
