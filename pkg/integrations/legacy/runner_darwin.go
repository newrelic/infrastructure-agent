// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import "os/exec"

// newCmd returns the Cmd struct to execute the given command with the provided
// arguments.
func (ep *externalPlugin) newCmd(executable string, args []string) *exec.Cmd {
	return exec.Command(executable, args...)
}
