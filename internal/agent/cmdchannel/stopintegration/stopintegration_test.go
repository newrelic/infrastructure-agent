// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package stopintegration

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/shirou/gopsutil/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	l = log.WithComponent("test")
)

func TestHandle_returnsErrorOnMissingPID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CC stop-intergation is not supported on Windows")
	}

	h := NewHandler(l)

	cmdArgsMissingPID := commandapi.Command{
		Args: []byte(`{ "integration_name": "foo" }`),
	}

	err := h.Handle(context.Background(), cmdArgsMissingPID, false)
	assert.Equal(t, cmdchannel.NewArgsErr(ErrNoIntPID).Error(), err.Error())
}

func TestHandle_signalStopProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CC stop-intergation is not supported on Windows")
	}

	// Given a running process
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proc := exec.CommandContext(ctx, "sleep", "5")
	pidC := make(chan int)
	go func() {
		require.NoError(t, proc.Start())
		pidC <- proc.Process.Pid
	}()

	h := NewHandler(l)

	pid := <-pidC
	cmd := commandapi.Command{
		Args: []byte(fmt.Sprintf(`{ "pid": %d }`, pid)),
	}

	p, err := process.NewProcess(int32(pid))
	require.NoError(t, err)

	st, err := p.StatusWithContext(ctx)
	require.NoError(t, err)
	if st != "S" && st != "R" {
		t.Fatal("sleep command should be either running or sleep")
	}

	// WHEN handler receives stop PID request
	err = h.Handle(context.Background(), cmd, false)
	require.NoError(t, err)

	// THEN process is stopped
	st, err = p.StatusWithContext(ctx)
	require.NoError(t, err)
	require.NotEqual(t, "S", st)
	require.NotEqual(t, "R", st)
}
