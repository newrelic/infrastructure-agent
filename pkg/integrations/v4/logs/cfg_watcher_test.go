// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	ctx2 "context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_HotReload_CreateAndModifyFile(t *testing.T) {
	ctx, cancel := ctx2.WithCancel(ctx2.Background())
	defer cancel()

	tempDir, err := ioutil.TempDir("", "test_agent")
	require.NoError(t, err)

	defer func() {
		err := os.RemoveAll(tempDir)
		require.NoError(t, err)
	}()

	// GIVEN a ConfigChangesWatcher on a temporary directory
	ccw := NewConfigChangesWatcher(tempDir)

	changes := make(chan struct{}, 100)
	ccw.Watch(ctx, changes)

	// WHEN file CREATE
	cfgFile := filepath.Join(tempDir, "test_agent.yaml")

	err = ioutil.WriteFile(cfgFile, []byte("test"), 0644)
	require.NoError(t, err)
	// THEN change discovered
	requireChanges(t, changes)

	// WHEN file CHANGED
	fh, err := os.OpenFile(cfgFile, os.O_APPEND|os.O_WRONLY, os.ModeAppend)

	require.NoError(t, err)

	_, err = fh.WriteString("test2")
	fh.Close()

	require.NoError(t, err)

	// THEN change discovered
	requireChanges(t, changes)

	// WHEN file REMOVED
	err = os.Remove(filepath.Join(tempDir, "test_agent.yaml"))
	require.NoError(t, err)

	// THEN change discovered
	requireChanges(t, changes)
}

func fileAppend(filePath, content string) error {
	fh, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer func() { _ = fh.Close() }()
	_, err = fh.WriteString(content)
	return err
}

func requireChanges(t *testing.T, changes chan struct{}) {
	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		require.Fail(t, "Timeout exceeded while waiting receiving a change signal")
	}
}
