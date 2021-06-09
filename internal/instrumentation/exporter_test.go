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

func TestNew_Measure(t *testing.T) {
	instruments, err := New()
	require.NoError(t, err)
	require.NotNil(t, instruments)

	ts := httptest.NewServer(instruments.GetHandler())
	defer ts.Close()

	for i := int64(1); i <= 100; i++ {
		instruments.Measure(Counter, DMRequestsForwarded, i)
	}
	for i := int64(1); i <= 200; i++ {
		instruments.Measure(Counter, DMDatasetsReceived, i)
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
	assert.Contains(t, string(metrics), "newrelic_infra_instrumentation_dm_requests_forwarded 5050")
	assert.Contains(t, string(metrics), "newrelic_infra_instrumentation_dm_datasets_received 20100")
}
