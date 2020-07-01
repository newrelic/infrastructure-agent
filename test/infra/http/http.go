// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package infra

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
)

const (
	body        = "response_foo"
	errorBody   = "response_error_foo"
	tooManyBody = "response_too_many_foo"
)

// RequestRecorderClient is an HTTP client that records the first arriving request.
type RequestRecorderClient struct {
	Client    backendhttp.Client
	RequestCh chan http.Request
}

type ResponseOption = func(http.Response)

// NewRequestRecorderClient creates a new RequestRecorderClient, passing the
// expected responses in order of invocation.
// If all the responses are returned or no responses are passed, it always
// returns "202 Status Accepted".
func NewRequestRecorderClient(responses ...http.Response) *RequestRecorderClient {
	requestCh := make(chan http.Request)

	responseIndex := 0

	httpClient := func(req *http.Request) (*http.Response, error) {
		requestCh <- *req

		if responseIndex < len(responses) {
			resp := responses[responseIndex]
			responseIndex++
			return &resp, nil
		}

		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(body))),
		}, nil
	}

	return &RequestRecorderClient{
		Client:    httpClient,
		RequestCh: requestCh,
	}
}

// AcceptedResponse creates a mock http.Response with a StatusAccepted code.
func AcceptedResponse(pluginId string, lastDeltaId int) http.Response {
	return http.Response{
		StatusCode: http.StatusAccepted,
		Body: ioutil.NopCloser(bytes.NewReader([]byte(
			fmt.Sprintf(`{"payload":{"version":1,"delta_state":{"%s":{"last_stored_id":%v,"send_next_id":%v}}}}`,
				pluginId, lastDeltaId, lastDeltaId+1)))),
	}
}

func ResetDeltasResponse(pluginId string) http.Response {
	return http.Response{
		StatusCode: http.StatusAccepted,
		Body: ioutil.NopCloser(bytes.NewReader([]byte(
			fmt.Sprintf(`{"payload":{"reset":"%v","version":1,"delta_state":{"%s":{"last_stored_id":%v,"send_next_id":%v}}}}`,
				inventoryapi.ResetAll, pluginId, 0, 1)))),
	}
}

// ErrorResponse simulates an error response.
var ErrorResponse = http.Response{
	StatusCode: http.StatusInternalServerError,
	Body:       ioutil.NopCloser(bytes.NewReader([]byte(errorBody))),
}

// RetryAfter adds the `Retry-After` header to the HTTP response.
func RetryAfter(value string) ResponseOption {
	return func(resp http.Response) {
		resp.Header.Add("Retry-After", value)
	}
}

// TooManyRequestsResponse creates a mock http.Response with a StatusTooManyRequests code.
func TooManyRequestsResponse(opts ...ResponseOption) http.Response {
	r := http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(tooManyBody))),
		Header:     http.Header{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}
