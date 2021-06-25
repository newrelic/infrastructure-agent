// Copyright 2021 NewServer Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package httpapi

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
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	network_helpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServe_Status(t *testing.T) {
	t.Parallel()

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
	em := &testemit.RecordEmitter{}
	s, err := NewServer(NewConfig(false, "", 0, true, port), r, em)
	require.NoError(t, err)
	defer cancel()

	go s.Serve(ctx)

	s.WaitUntilReady()

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

func TestServe_OnlyErrors(t *testing.T) {
	t.Parallel()

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
	em := &testemit.RecordEmitter{}
	s, err := NewServer(NewConfig(false, "", 0, true, port), r, em)
	defer cancel()

	go s.Serve(ctx)

	s.WaitUntilReady()

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

func TestServe_IngestData(t *testing.T) {
	t.Parallel()

	port, err := network_helpers.TCPPort()
	require.NoError(t, err)

	em := &testemit.RecordEmitter{}
	s, err := NewServer(NewConfig(true, "localhost", port, false, 0), &noopReporter{}, em)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.Serve(ctx)

	payloadWritten := make(chan struct{})
	go func() {
		s.WaitUntilReady()
		client := http.Client{}
		postReq, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d%s", port, ingestAPIPath), bytes.NewReader(fixtures.FooBytes))
		resp, err := client.Do(postReq)
		require.NoError(t, err)
		require.Equal(t, 20, resp.StatusCode/10, "status code: %v", resp.StatusCode)
		close(payloadWritten)
	}()

	select {
	case <-time.NewTimer(1000 * time.Millisecond).C:
		t.Error("timeout waiting for HTTP request to be submitted")
	case <-payloadWritten:
	}

	t.Log("receiving from integration...\n")
	d, err := em.ReceiveFrom(IntegrationName)
	require.NoError(t, err)
	assert.Equal(t, "unique foo", d.DataSet.PluginDataSet.Entity.Name)
}

type noopReporter struct{}

func (r *noopReporter) Report() (status.Report, error) {
	return status.Report{}, nil
}

func (r *noopReporter) ReportErrors() (status.Report, error) {
	return status.Report{}, nil
}
