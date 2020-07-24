// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package identity

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAPIClient_FailedInLocal(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ConnectResponse{}

		js, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}))
	defer ts.Close()

	configuration := NewConfiguration()
	configuration.BasePath = ts.URL

	ac := NewAPIClient(configuration)
	connectResponse, httpResponse, err := ac.DefaultApi.ConnectPost(
		context.Background(), "user-agent", "license-key",
		ConnectRequest{
			Fingerprint: Fingerprint{
				FullHostname: "foo.example.org",
				Hostname: "foo",
			},
		}, nil)

	assert.NoError(t, err)
	assert.Empty(t, connectResponse)
	assert.EqualValues(t, "200 OK", httpResponse.Status)
}
