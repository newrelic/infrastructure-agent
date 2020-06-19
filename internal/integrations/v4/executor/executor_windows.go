// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package executor

import (
	"context"
	"os/exec"
)

// userAwareCmd returns a cancellable Cmd struct to execute the given command with the provided
// arguments.
func (r *Executor) userAwareCmd(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, r.Command, r.Args...)
}
