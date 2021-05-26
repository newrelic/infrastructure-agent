// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package statusapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	network_helpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_Serve(t *testing.T) {
	// Given a running HTTP endpoint
	port, err := network_helpers.TCPPort()
	require.NoError(t, err)

	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer serverOk.Close()

	// And a status reporter monitoring it
	endpoints := []string{serverOk.URL}
	l := log.WithComponent(t.Name())
	timeout := 10 * time.Millisecond
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := status.NewReporter(ctx, l, endpoints, timeout, transport, emptyIDProvide, "user-agent", "agent-key")

	// When agent status API server is ready
	s := NewServer(port, r)
	defer cancel()

	go s.Serve(ctx)

	s.WaitUntilReady()
	time.Sleep(10 * time.Millisecond)

	// And a request to the status API is sent
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, statusAPIPath), bytes.NewReader([]byte{}))
	require.NoError(t, err)
	client := http.Client{}

	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	// Then response contains a report for the monitored endpoint
	require.Equal(t, http.StatusOK, res.StatusCode)

	var gotReport status.Report
	json.NewDecoder(res.Body).Decode(&gotReport)
	require.Len(t, gotReport.Checks.Endpoints, 1)
	e := gotReport.Checks.Endpoints[0]
	assert.Empty(t, e.Error)
	assert.True(t, e.Reachable)
	assert.Equal(t, serverOk.URL, e.URL)
}

func TestServer_Serve_OnlyErrors(t *testing.T) {
	// Given a running HTTP endpoint and an errored one (which times out)
	port, err := network_helpers.TCPPort()
	require.NoError(t, err)

	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer serverOk.Close()
	serverTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer serverTimeout.Close()

	// And a status reporter monitoring these endpoints
	endpoints := []string{serverOk.URL, serverTimeout.URL}
	l := log.WithComponent(t.Name())
	timeout := 10 * time.Millisecond
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := status.NewReporter(ctx, l, endpoints, timeout, transport, emptyIDProvide, "user-agent", "agent-key")

	// When agent status API server is ready
	s := NewServer(port, r)
	defer cancel()

	go s.Serve(ctx)

	s.WaitUntilReady()
	time.Sleep(10 * time.Millisecond)

	// And a request to the status API is sent
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, statusOnlyErrorsAPIPath), bytes.NewReader([]byte{}))
	require.NoError(t, err)
	client := http.Client{}

	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	// Then response contains a report for the monitored endpoint
	require.Equal(t, http.StatusOK, res.StatusCode)

	var gotReport status.Report
	json.NewDecoder(res.Body).Decode(&gotReport)
	require.Len(t, gotReport.Checks.Endpoints, 1, "only errored endpoint should be reported")
	e := gotReport.Checks.Endpoints[0]
	assert.NotEmpty(t, e.Error)
	assert.False(t, e.Reachable)
	assert.Equal(t, serverTimeout.URL, e.URL)
}
