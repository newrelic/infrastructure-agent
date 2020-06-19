// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package sender

import (
	"context"
	"errors"
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/ipc"

	"github.com/newrelic/infrastructure-agent/internal/os/api/signals"

	"os"
	"syscall"
)

// RequiredPIDErr is the error to return when the process ID is not provided.
var RequiredPIDErr = errors.New("process PID of the agent is required")

// unix process based agent control client.
type unixClient struct {
	proc *os.Process
}

// NewClient creates a unix process based agent control client.
func NewClient(agentPID int) (c Client, err error) {
	if agentPID == 0 {
		err = RequiredPIDErr
		return
	}

	p, err := os.FindProcess(agentPID)
	if err != nil {
		err = fmt.Errorf("cannot find process for PID %d", agentPID)
		return

		// check if process is alive
	} else if err = p.Signal(syscall.Signal(0)); err != nil {
		err = fmt.Errorf("signaling 0 on pid %d returned: %s", agentPID, err)
		return
	}

	c = &unixClient{
		proc: p,
	}

	return
}

// Notify will notify a running agent process by sending a signal to the process.
func (c *unixClient) Notify(_ context.Context, _ ipc.Message) error {
	if err := c.proc.Signal(signals.Notification); err != nil {
		return fmt.Errorf("cannot signal process %d", c.proc.Pid)
	}

	return nil
}

// Return the identification for the notified agent.
func (c *unixClient) GetID() string {
	return fmt.Sprintf("%d", c.proc.Pid)
}
