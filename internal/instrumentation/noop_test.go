// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNoopServer(t *testing.T) {
	server := NewNoopServer()
	require.NotNil(t, server)
}

func TestNoop_GetHandler(t *testing.T) {
	ts := httptest.NewServer(NewNoopServer().GetHandler())
	defer ts.Close()
	res, err := http.Get(ts.URL)
	require.NoError(t, err)

	_, err = ioutil.ReadAll(res.Body)
	_ = res.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

func TestNoop_GetHttpTransport(t *testing.T) {
	transport := &http.Transport{MaxIdleConnsPerHost: 1}
	actualTransport := NewNoopServer().GetHttpTransport(transport)
	assert.Equal(t, transport, actualTransport)
}
