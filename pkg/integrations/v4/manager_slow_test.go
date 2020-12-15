// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build slow

package v4

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	definitionQ = make(chan integration.Definition, 1000)
)

// this test is used to make sure we see file changes on K8s
// slow test
func TestManager_HotReload_CreateAndModifyLinkFile(t *testing.T) {
	skipIfWindows(t)
	// GIVEN an integration
	dir, err := tempFiles(map[string]string{
		"integration": v4AppendableConfig,
	})

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(Configuration{ConfigFolders: []string{dir}}, emitter, integration.ErrLookup, definitionQ, track.NewTracker())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THAT is correctly running
	// (expect nothing)
	expectNoMetric(t, emitter, "hotreload-test")

	// THEN create integration

	require.NoError(t, err)
	err = os.Rename(filepath.Join(dir, "integration"), filepath.Join(dir, "first_config"))
	require.NoError(t, err)

	err = os.Link(filepath.Join(dir, "first_config"), filepath.Join(dir, "integration.yaml"))
	require.NoError(t, err)

	// THEN the integration is picked yo
	testhelpers.Eventually(t, 15*time.Second, func(t require.TestingT) {
		metric := expectOneMetric(t, emitter, "hotreload-test")
		require.Equal(t, "first", metric["value"])
	})

	// (then returns a value passed by argument, or "unset" if not set)
	metric := expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "unset", metric["value"])

	// WHEN we modify the integration file at runtime by changing symlink
	bs, err := ioutil.ReadFile(filepath.Join(dir, "first_config"))
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(dir, "second_config"), bs, 0644))
	require.NoError(t, fileAppend(
		filepath.Join(dir, "second_config"),
		"      - modifiedValue\n"))
	require.NoError(t,
		os.Remove(filepath.Join(dir, "integration.yaml")))
	require.NoError(t,
		os.Link(filepath.Join(dir, "second_config"), filepath.Join(dir, "integration.yaml")))

	// THEN the integration is restarted
	testhelpers.Eventually(t, 15*time.Second, func(t require.TestingT) {
		// waiting to empty the previous process queue and receive a "first" value again
		metric = expectOneMetric(t, emitter, "hotreload-test")
		require.Equal(t, "first", metric["value"])
	})
	// AND the integration reflects the changes in the configuration file
	metric = expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "modifiedValue", metric["value"])
}

func TestLongRunning_HeartBeat(t *testing.T) {
	// GIVEN a long running integration sending a heartbeat
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(t, err)
	require.NoError(t, testhelp.GoBuild(fixtures.LongRunningHBGoFile, filepath.Join(niDir, "heartbeating"+fixtures.CmdExtension)))

	// AND a v4 configuration file specifying a timeout larger than this heartbeat
	// but lower than the metrics submission rate
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `---
integrations:
  - name: heartbeating
    timeout: 1s
`})
	require.NoError(t, err)

	// WHEN the v4 integrations manager runs it
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(Configuration{
		ConfigFolders:     []string{configDir},
		DefinitionFolders: []string{niDir},
	}, emitter, integration.ErrLookup, definitionQ, track.NewTracker())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	metric := expectOneMetric(t, emitter, "heartbeating")
	require.Equal(t, "hello", metric["value"])

	// THEN the integration is not killed and continues sending metrics
	stopTesting := make(chan struct{})
	receivedMetrics := int64(0)
	go func() {
		select {
		case <-stopTesting:
			return
		default:
			metric := expectOneMetric(t, emitter, "heartbeating")
			require.Equal(t, "hello", metric["value"])
			// using atomics to avoid race conditions false positives during -race tests
			atomic.AddInt64(&receivedMetrics, 1)
		}
	}()
	<-time.After(3 * time.Second)
	close(stopTesting)
	assert.NotZero(t, atomic.LoadInt64(&receivedMetrics))
}
