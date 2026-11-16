// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestJobObject_AddProcess_ProcessIsTerminatedWhenJobIsClosed checks that a process added to the job is
// terminated when the job is closed, which is what happens to the agent's helper processes when the
// service process ends. It guards AddProcess against depending on the Go version's os.Process layout.
func TestJobObject_AddProcess_ProcessIsTerminatedWhenJobIsClosed(t *testing.T) {
	t.Parallel()

	job, err := NewJob()
	require.NoError(t, err)

	// cmd.exe waits for input on its stdin, so it keeps running until it is terminated.
	cmd := exec.CommandContext(t.Context(), "cmd.exe", "/c", "pause")

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)

	t.Cleanup(func() { _ = stdin.Close() })

	require.NoError(t, cmd.Start())

	exited := make(chan error, 1)

	go func() { exited <- cmd.Wait() }()

	require.NoError(t, job.AddProcess(cmd.Process))
	require.NoError(t, job.Close())

	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatal("the process was not terminated when the job object was closed")
	}
}
