// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package infra

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

type postConnectResponse struct {
	Identity entityIDResponse `json:"identity"`
}

type entityIDResponse struct {
	EntityId entity.ID `json:"entityId"`
}

func SuccessConnectClient(req *http.Request) (*http.Response, error) {
	body := &postConnectResponse{
		Identity: entityIDResponse{
			EntityId: entity.ID(12345),
		},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	closer := ioutil.NopCloser(bytes.NewReader(buf))
	return &http.Response{
		Header:     http.Header{},
		StatusCode: http.StatusCreated,
		Body:       closer,
	}, nil
}

func FailingConnectClient(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Header:     http.Header{},
		StatusCode: http.StatusInternalServerError,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte("Fatal error from platform"))),
	}, nil
}
