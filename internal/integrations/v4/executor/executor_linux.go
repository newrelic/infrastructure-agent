// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package executor

import (
	"context"
	"os/exec"
)

// userAwareCmd returns a cancellable Cmd struct to execute the given command with the provided
// arguments. If the plugin instance contains a value for IntegrationUser the
// command will be constructed with sudo to allow it to be run as the specified
// user.
func (r *Executor) userAwareCmd(ctx context.Context) *exec.Cmd {
	if r.Cfg.User == "" {
		return exec.CommandContext(ctx, r.Command, r.Args...)
	}
	// The -n flag makes sudo fail, if a password is required, with the
	// following message: `sudo: a password is required`.
	sudoArgs := append(
		[]string{"-E", "-n", "-u", r.Cfg.User, r.Command},
		r.Args...,
	)
	return exec.CommandContext(ctx, "/usr/bin/sudo", sudoArgs...)
}

func startProcess(cmd *exec.Cmd) error {
	return cmd.Start()
}
