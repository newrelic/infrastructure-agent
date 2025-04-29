// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package executor

import (
	"context"
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows"
)

// userAwareCmd returns a cancellable Cmd struct to execute the given command with the provided
// arguments.
func (r *Executor) userAwareCmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, r.Command, r.Args...)
}

// startProcess starts and sets priority to command
func startProcess(cmd *exec.Cmd) error {
	err := cmd.Start()
	if err != nil {
		return err
	}

	err = setPriorityClass(cmd)
	if err != nil {
		illog.WithError(err).WithField("command", cmd.String()).Error("cannot set priority class to process")
	}

	return nil
}

// setPriorityClass will set the priorityClass of the agent to the cmd process
func setPriorityClass(cmd *exec.Cmd) error {
	priorityClass, err := windows.GetPriorityClass(windows.CurrentProcess())
	if err != nil {
		return fmt.Errorf("fail to get priorityClass from current process: %w", err)
	}

	handle, err := processHandle(cmd)
	if err != nil {
		return fmt.Errorf("failed to get proc handle: %v", err)
	}

	return windows.SetPriorityClass(handle, priorityClass)
}

// processHandle returns windows handle from cmd
func processHandle(cmd *exec.Cmd) (windows.Handle, error) {
	var handle windows.Handle
	if cmd.Process == nil {
		return handle, fmt.Errorf("process cannot be nil pointer")
	}

	// Avoid Unsafe handle access to obtain a fresh, valid handle for the process
	// Add nolint to avoid G115 false positive
	handle, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION|windows.PROCESS_QUERY_INFORMATION, false, uint32(cmd.Process.Pid)) //nolint:gosec

	if err != nil {
		return 0, fmt.Errorf("open process failed : %w", err) //nolint:wrapcheck
	}

	return handle, nil
}
