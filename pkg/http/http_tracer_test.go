// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"github.com/newrelic/infrastructure-agent/pkg/log"
	log2 "github.com/newrelic/infrastructure-agent/test/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithTracer(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	request, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	request = WithTracer(request, "test")

	hook := log2.NewInMemoryEntriesHook([]logrus.Level{logrus.DebugLevel})
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	_, err = server.Client().Do(request)
	require.NoError(t, err)
	expectedEntries := []logrus.Fields{
		{"action": "GetConn", "component": "HttpTracer", "requester": "test"},
		{"action": "ConnectStart", "component": "HttpTracer", "requester": "test"},
		{"action": "ConnectDone", "component": "HttpTracer", "requester": "test"},
		{"action": "GotConn", "component": "HttpTracer", "requester": "test"},
		{"action": "WroteHeaders", "component": "HttpTracer", "requester": "test"},
		{"action": "WroteRequest", "component": "HttpTracer", "requester": "test"},
		{"action": "GotFirstResponseByte", "component": "HttpTracer", "requester": "test"},
	}
	entries := hook.GetEntries()
	for i, entry := range entries {
		assert.Equal(t, expectedEntries[i]["action"], entry.Data["action"])
		assert.Equal(t, expectedEntries[i]["component"], entry.Data["component"])
		assert.Equal(t, expectedEntries[i]["requester"], entry.Data["requester"])
	}

}
