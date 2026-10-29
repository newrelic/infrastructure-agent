// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func newTestReporter(t *testing.T) (*Reporter, string) {
	t.Helper()

	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	now := func() time.Time { return time.Unix(0, 2) }

	return NewReporter("nri-apache.yaml", w, time.Unix(0, 1), now), filepath.Join(dir, "nri-apache.yaml")
}

func readReport(t *testing.T, path string) Report {
	t.Helper()

	contents, err := os.ReadFile(path)
	require.NoError(t, err)

	var r Report
	require.NoError(t, yaml.Unmarshal(contents, &r))

	return r
}

// The keys are the contract with Agent Control's file health checker.
func TestWriter_Write_producesTheExpectedKeys(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	require.NoError(t, w.Write("nri-apache.yaml", Report{
		Healthy:            false,
		Status:             "nri-apache: no data",
		LastError:          noDataLastError,
		StartTimeUnixNano:  1000,
		StatusTimeUnixNano: 2000,
	}))

	contents, err := os.ReadFile(filepath.Join(dir, "nri-apache.yaml"))
	require.NoError(t, err)

	assert.Equal(t, "healthy: false\n"+
		"status: 'nri-apache: no data'\n"+
		"last_error: integration ran without producing metrics, events or inventory\n"+
		"start_time_unix_nano: 1000\n"+
		"status_time_unix_nano: 2000\n", string(contents))
}

func TestNewWriter_rejectsAnEmptyDir(t *testing.T) {
	_, err := NewWriter("")

	require.ErrorIs(t, err, ErrEmptyDir)
}

func TestWriter_Write_leavesNoTemporaryFileBehind(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	require.NoError(t, w.Write("nri-apache.yaml", Report{Healthy: true}))
	require.NoError(t, w.Write("nri-apache.yaml", Report{Healthy: false}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "nri-apache.yaml", entries[0].Name())
}

// The health dir can be removed under the agent's feet; the error must reach the caller.
func TestWriter_Write_returnsAnErrorWhenTheDirIsGone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "health")
	w, err := NewWriter(dir)
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(dir))

	require.Error(t, w.Write("nri-apache.yaml", Report{Healthy: true}))
}

func TestWriter_Remove_isIdempotent(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	require.NoError(t, err)

	require.NoError(t, w.Write("nri-apache.yaml", Report{Healthy: true}))
	require.NoError(t, w.Remove("nri-apache.yaml"))
	require.NoError(t, w.Remove("nri-apache.yaml"))
}

func TestReporter_Init_writesAHealthyReportBeforeAnythingRan(t *testing.T) {
	reporter, path := newTestReporter(t)

	require.NoError(t, reporter.Init())

	report := readReport(t, path)
	assert.True(t, report.Healthy)
	assert.Equal(t, statusNotRunYet, report.Status)
	assert.Empty(t, report.LastError)
	assert.Equal(t, int64(1), report.StartTimeUnixNano)
	assert.Equal(t, int64(2), report.StatusTimeUnixNano)
}

func TestReporter_Record(t *testing.T) {
	testCases := []struct {
		name              string
		result            Result
		expectedHealthy   bool
		expectedStatus    string
		expectedLastError string
	}{
		{
			name:            "data produced",
			result:          Result{Executions: 1, DataPayloads: 1},
			expectedHealthy: true,
			expectedStatus:  statusAllHealthy,
		},
		{
			name:              "execution failed",
			result:            Result{Executions: 1, Errors: []string{"exit status 3"}},
			expectedHealthy:   false,
			expectedStatus:    "nri-apache: execution failed",
			expectedLastError: "exit status 3",
		},
		{
			name:              "ran and emitted nothing",
			result:            Result{Executions: 1, DataPayloads: 0},
			expectedHealthy:   false,
			expectedStatus:    "nri-apache: no data",
			expectedLastError: noDataLastError,
		},
		{
			// no discovery match or unmet "when" conditions: the integration is not broken
			name:            "nothing ran",
			result:          Result{},
			expectedHealthy: true,
			expectedStatus:  statusAllHealthy,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reporter, path := newTestReporter(t)

			require.NoError(t, reporter.Record("hash-1", "nri-apache", testCase.result))

			report := readReport(t, path)
			assert.Equal(t, testCase.expectedHealthy, report.Healthy)
			assert.Equal(t, testCase.expectedStatus, report.Status)
			assert.Equal(t, testCase.expectedLastError, report.LastError)
		})
	}
}

func TestReporter_Record_oneUnhealthyIntegrationMakesTheFileUnhealthy(t *testing.T) {
	reporter, path := newTestReporter(t)

	require.NoError(t, reporter.Record("hash-1", "nri-apache", Result{Executions: 1, DataPayloads: 1}))
	require.NoError(t, reporter.Record("hash-2", "nri-nginx", Result{Executions: 1, Errors: []string{"boom"}}))

	report := readReport(t, path)
	assert.False(t, report.Healthy)
	assert.Equal(t, "nri-nginx: execution failed", report.Status)
	assert.Equal(t, "boom", report.LastError)
}

func TestReporter_Record_recoveryClearsTheLastError(t *testing.T) {
	reporter, path := newTestReporter(t)

	require.NoError(t, reporter.Record("hash-1", "nri-apache", Result{Executions: 1, Errors: []string{"boom"}}))
	require.False(t, readReport(t, path).Healthy)

	require.NoError(t, reporter.Record("hash-1", "nri-apache", Result{Executions: 1, DataPayloads: 1}))

	report := readReport(t, path)
	assert.True(t, report.Healthy)
	assert.Empty(t, report.LastError)
}

// With two integrations unhealthy at once the reported one must not depend on map iteration order,
// or the status flaps between two values on every write.
func TestReporter_Record_reportsTheSameIntegrationWhenSeveralAreUnhealthy(t *testing.T) {
	reporter, path := newTestReporter(t)

	require.NoError(t, reporter.Record("hash-z", "nri-nginx", Result{Executions: 1, Errors: []string{"nginx is down"}}))
	require.NoError(t, reporter.Record("hash-a", "nri-apache", Result{Executions: 1, Errors: []string{"apache is down"}}))

	for i := 0; i < 20; i++ {
		require.NoError(t, reporter.Init())
		assert.Equal(t, "nri-apache: execution failed", readReport(t, path).Status)
	}
}

func TestHasData(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		payload  string
		expected bool
	}{
		{
			name:     "protocol v1 with metrics at the top level",
			payload:  `{"name":"com.newrelic.test","protocol_version":"1","metrics":[{"event_type":"TestSample"}]}`,
			expected: true,
		},
		{
			name:     "protocol v1 with an empty metrics array",
			payload:  `{"name":"com.newrelic.test","protocol_version":"1","metrics":[]}`,
			expected: false,
		},
		{
			name:     "protocol v1 with inventory only",
			payload:  `{"name":"com.newrelic.test","protocol_version":"1","inventory":{"apache":{"version":"2.4"}}}`,
			expected: true,
		},
		{
			name:     "protocol v3 with metrics in a data set",
			payload:  `{"name":"com.newrelic.apache","protocol_version":"3","data":[{"metrics":[{"event_type":"ApacheSample"}]}]}`,
			expected: true,
		},
		{
			name:     "protocol v3 cleared by Integration.Clear",
			payload:  `{"name":"com.newrelic.apache","protocol_version":"3","data":[]}`,
			expected: false,
		},
		{
			name:     "protocol v3 with a data set that holds nothing",
			payload:  `{"name":"com.newrelic.apache","protocol_version":"3","data":[{"metrics":[],"inventory":{},"events":[]}]}`,
			expected: false,
		},
		{
			name:     "protocol v3 with events in the second data set only",
			payload:  `{"name":"com.newrelic.apache","protocol_version":"3","data":[{"metrics":[]},{"events":[{"summary":"x"}]}]}`,
			expected: true,
		},
		{
			name:     "protocol v4 with metrics",
			payload:  `{"protocol_version":"4","integration":{"name":"com.newrelic.apache"},"data":[{"metrics":[{"name":"apache.busyWorkers"}]}]}`,
			expected: true,
		},
		{
			name:     "not JSON at all",
			payload:  `starting`,
			expected: false,
		},
		{
			name:     "empty payload",
			payload:  ``,
			expected: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, HasData([]byte(testCase.payload)))
		})
	}
}

// The integrations declared in one configuration file are run by concurrent runners that share a
// single Reporter, so their rounds overlap. Whatever the interleaving, once every Record has
// returned the file must show the unhealthy verdict: a report computed before the failure was
// recorded must never be the one left on disk.
func TestReporter_Record_concurrentRecordsNeverLoseTheUnhealthyVerdict(t *testing.T) {
	t.Parallel()

	const (
		integrations = 6
		rounds       = 100
	)

	for round := 0; round < rounds; round++ {
		reporter, path := newTestReporter(t)

		start := make(chan struct{})

		var wg sync.WaitGroup

		wg.Add(integrations)

		for i := 0; i < integrations; i++ {
			go func(i int) {
				defer wg.Done()

				result := Result{Executions: 1, DataPayloads: 1}
				if i == 0 {
					result = Result{Executions: 1, Errors: []string{"boom"}}
				}

				<-start

				assert.NoError(t, reporter.Record(fmt.Sprintf("hash-%d", i), fmt.Sprintf("nri-%d", i), result))
			}(i)
		}

		close(start)
		wg.Wait()

		if report := readReport(t, path); report.Healthy {
			t.Fatalf("round %d: the recorded failure was lost, file reports %+v", round, report)
		}
	}
}
