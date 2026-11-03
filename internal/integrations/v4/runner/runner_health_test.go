// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/health"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// failingEmitter rejects every payload with a fixed error.
type failingEmitter struct {
	err error
}

func (f failingEmitter) Emit(_ integration.Definition, _ data.Map, _ []data.EntityRewrite, _ []byte) error {
	return f.err
}

func runWithHealthReporter(t *testing.T, entry config.ConfigEntry) health.Report {
	t.Helper()

	return runWithHealthReporterAndEmitter(t, entry, &testemit.RecordEmitter{})
}

func runWithHealthReporterAndEmitter(t *testing.T, entry config.ConfigEntry, e emitter.Emitter) health.Report {
	t.Helper()

	dir := t.TempDir()
	writer, err := health.NewWriter(dir)
	require.NoError(t, err)

	def, err := integration.NewDefinition(entry, integration.ErrLookup, nil, nil)
	require.NoError(t, err)

	r := NewRunner(def, e, nil, nil, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, nil, host.IDLookup{}).
		WithHealthReporter(health.NewReporter("nri-test.yaml", writer, time.Unix(0, 1), time.Now))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r.Run(ctx, nil, nil)

	contents, err := os.ReadFile(filepath.Join(dir, "nri-test.yaml"))
	require.NoError(t, err)

	var report health.Report
	require.NoError(t, yaml.Unmarshal(contents, &report))

	return report
}

func Test_runner_health_dataProducedIsHealthy(t *testing.T) {
	report := runWithHealthReporter(t, config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.IntegrationScript, "bar"),
	})

	assert.True(t, report.Healthy)
	assert.Equal(t, "All integrations healthy", report.Status)
	assert.Empty(t, report.LastError)
}

func Test_runner_health_nonZeroExitIsUnhealthy(t *testing.T) {
	report := runWithHealthReporter(t, config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.ErrorCmd),
	})

	assert.False(t, report.Healthy)
	assert.Equal(t, "foo: execution failed", report.Status)
	assert.NotEmpty(t, report.LastError)
}

// The health file is forwarded to a supervisor, so an error carrying a secret must not reach
// last_error verbatim.
func Test_runner_health_obfuscatesSecretsInTheRecordedError(t *testing.T) {
	report := runWithHealthReporterAndEmitter(t, config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.IntegrationScript, "bar"),
	}, failingEmitter{err: errors.New(`cannot emit payload {"password":"s3cr3t-value"}`)})

	assert.False(t, report.Healthy)
	assert.Equal(t, "foo: execution failed", report.Status)
	assert.NotContains(t, report.LastError, "s3cr3t-value")
	assert.Contains(t, report.LastError, "cannot emit payload")
}

// The integrations declared in one configuration file are run by concurrent runners that share a
// single health reporter. A failure in any of them must surface in the file, and must keep showing
// there while the others go on succeeding.
func Test_group_health_oneFailingIntegrationMakesTheFileUnhealthy(t *testing.T) {
	// registered first, so it runs after cancel below and waits for the runners to stop writing
	// before the temporary directory is removed
	defer leaktest.Check(t)()

	dir := t.TempDir()
	writer, err := health.NewWriter(dir)
	require.NoError(t, err)

	loader := NewLoadFn(config.YAML{
		Integrations: []config.ConfigEntry{
			{InstanceName: "healthy-one", Exec: testhelp.Command(fixtures.IntegrationScript, "hello")},
			{InstanceName: "failing-one", Exec: testhelp.Command(fixtures.ErrorCmd)},
		},
	}, nil)

	gr, _, err := NewGroup(loader, integration.InstancesLookup{}, nil, &testemit.RecordEmitter{},
		cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, "", terminatedQueue, host.IDLookup{})
	require.NoError(t, err)

	gr.SetHealthReporter(health.NewReporter("nri-test.yaml", writer, time.Unix(0, 1), time.Now))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(dir, "nri-test.yaml")

	// the file exists as soon as the configuration file is loaded, before anything has run
	require.True(t, gr.Run(ctx))
	require.FileExists(t, path)

	require.Eventually(t, func() bool {
		contents, err := os.ReadFile(path)
		if err != nil {
			return false
		}

		var report health.Report
		if err := yaml.Unmarshal(contents, &report); err != nil {
			return false
		}

		return !report.Healthy && report.Status == "failing-one: execution failed"
	}, 10*time.Second, 10*time.Millisecond, "the failure of one integration never reached the shared health file")
}
