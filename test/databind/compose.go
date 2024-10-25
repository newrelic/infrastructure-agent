// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"fmt"
	"os"
	"os/exec"
)

// ComposeUp builds in the background the `docker-compose.yml` file that is passed as argument. It returns a
// function that must be invoked to destroy the spawned cluster.
func ComposeUp(file string) error {
	cmd := exec.Command("docker", "compose", "-f", file, "up", "--build", "-d")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s -> %s", err.Error(), string(output))
	}
	return nil
}

// ComposeDown destroys the cluster defined in the file. Not usually invoked.
func ComposeDown(file string) {
	cmd := exec.Command("docker", "compose", "-f", file, "down")
	output, err := cmd.CombinedOutput()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s -> %s", err.Error(), string(output))
		os.Exit(-1)
	}
}

// Exec executes a command inside a container, and returns its combined output.
func Exec(container string, cmdLine ...string) (string, error) {
	parts := []string{"exec", "-t", container}
	parts = append(parts, cmdLine...)
	cmd := exec.Command("docker", parts...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
