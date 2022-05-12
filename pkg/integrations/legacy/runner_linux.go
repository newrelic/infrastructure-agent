// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import "os/exec"

// newCmd returns the Cmd struct to execute the given command with the provided
// arguments. If the plugin instance contains a value for IntegrationUser the
// command will be constructed with sudo to allow it to be run as the specified
// user.
func (ep *externalPlugin) newCmd(executable string, args []string) *exec.Cmd {
	integrationUser := ep.pluginInstance.IntegrationUser
	if integrationUser == "" {
		return exec.Command(executable, args...)
	}
	// The -n flag makes sudo fail, if a password is required, with the
	// following message: `sudo: a password is required`.
	sudoArgs := append(
		[]string{"-E", "-n", "-u", integrationUser, executable},
		args...,
	)
	return exec.Command("/bin/sudo", sudoArgs...)
}
