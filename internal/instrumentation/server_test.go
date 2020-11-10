// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build go1.13

package instrumentation

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpentelemetryServer(t *testing.T) {
	exporter, err := NewOpentelemetryServer()
	require.NoError(t, err)
	require.NotNil(t, exporter)

	ts := httptest.NewServer(exporter.GetHandler())
	defer ts.Close()

	for i := int64(1); i <= 100; i++ {
		exporter.IncrementSomething(i)
	}

	metricsUrl := ts.URL + "/metrics"
	t.Logf("metricsUrl: %v", metricsUrl)
	res, err := http.Get(metricsUrl)
	require.NoError(t, err)

	metrics, err := ioutil.ReadAll(res.Body)
	_ = res.Body.Close()
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	assert.Contains(t, string(metrics), "go_gc_duration_seconds")
	assert.Contains(t, string(metrics), "newrelic_infra_instrumentation_counter 5050")
	t.Logf("%s", metrics)
}
