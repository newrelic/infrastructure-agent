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
	port, err := network_helpers.TCPPort()
	require.NoError(t, err)

	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer serverOk.Close()

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

	s := NewServer(port, r)
	defer cancel()

	go s.Serve(ctx)

	s.WaitUntilReady()
	time.Sleep(10 * time.Millisecond)

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, statusAPIPath), bytes.NewReader([]byte{}))
	require.NoError(t, err)
	client := http.Client{}

	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	var gotReport status.Report
	json.NewDecoder(res.Body).Decode(&gotReport)
	require.Len(t, gotReport.Endpoints, 1)
	e := gotReport.Endpoints[0]
	assert.Empty(t, e.Error)
	assert.True(t, e.Reachable)
	assert.Equal(t, serverOk.URL, e.URL)
}
