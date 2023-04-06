// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Command struct {
	CmdPath string   `yaml:"path"`
	CmdArgs []string `yaml:"args,omitempty"`
}

type commandGatherer struct {
	cfg *Command
}

var ErrNoPath = errors.New("command secrets must have a path parameter in order to be set")

func validationError(err error) error {
	return fmt.Errorf("validation error: %w", err)
}

func runCommandError(err error) error {
	return fmt.Errorf("failed to run command: %w", err)
}

func (c *Command) Validate() error {
	if c.CmdPath == "" {
		return validationError(ErrNoPath)
	}

	return nil
}

// CommandGatherer instantiates a Command variable gatherer from the given configuration. The fetching process
// will return either a map containing access paths to the stored JSON or a byte string value
// E.g. if the stored Secret is `{"account":{"user":"test1","password":"test2"}}`, the returned Map
// contents will be:
// "account.user"     -> "test1"
// "account.password" -> "test2".
func CommandGatherer(cmd *Command) func() (any, error) {
	cfg := commandGatherer{cmd}

	return func() (any, error) {
		dt, err := cfg.get()
		if err != nil {
			return nil, err
		}

		return dt, err
	}
}

func (c *commandGatherer) get() (any, error) {
	res, err := runCommand(c.cfg.CmdPath, c.cfg.CmdArgs)
	if err != nil {
		return nil, err
	}

	result := map[string]any{}
	if err := json.Unmarshal(res, &result); err != nil {
		slog.WithError(err).Debugf("failed converting command output to json: %v. Returning string", err)

		return strings.TrimSpace(string(res)), nil
	}

	return result, nil
}

// runCommand executes the given command and returns the contents of `stdout`.
func runCommand(path string, args []string) ([]byte, error) {
	_, err := exec.LookPath(path)
	if err != nil {
		return nil, runCommandError(err)
	}

	res, err := exec.Command(path, args...).Output()
	if err != nil {
		return nil, runCommandError(err)
	}

	return res, nil
}
