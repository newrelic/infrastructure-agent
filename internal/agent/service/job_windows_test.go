// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startLongRunningChild starts a process that keeps running until it is killed, and returns a
// channel that receives when the process has exited.
func startLongRunningChild(t *testing.T) (*exec.Cmd, <-chan error) {
	t.Helper()

	cmd := exec.Command("ping", "-n", "300", "127.0.0.1")
	require.NoError(t, cmd.Start())

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})

	return cmd, exited
}

func TestJobObject_AddProcess_ChildIsKilledWhenJobIsClosed(t *testing.T) {
	t.Parallel()

	job, err := NewJob()
	require.NoError(t, err)

	cmd, exited := startLongRunningChild(t)

	// This is what newrelic-infra-service does with the agent process. It must work whatever the
	// Go version's os.Process layout is.
	require.NoError(t, job.AddProcess(cmd.Process))

	select {
	case <-exited:
		t.Fatal("the child must keep running while the job is open")
	case <-time.After(500 * time.Millisecond):
	}

	// Closing the job (what happens when the service process ends, or the agent is killed) must
	// terminate everything that was assigned to it.
	require.NoError(t, job.Close())

	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatal("the child process was not terminated when the job object was closed")
	}
}

func TestJobObject_AddProcess_FailsForUnknownProcess(t *testing.T) {
	t.Parallel()

	job, err := NewJob()
	require.NoError(t, err)

	t.Cleanup(func() { _ = job.Close() })

	cmd, exited := startLongRunningChild(t)
	require.NoError(t, cmd.Process.Kill())
	<-exited

	// The process is gone: the error must be reported and not silently ignored.
	err = job.AddProcess(cmd.Process)
	assert.Error(t, err)
}
