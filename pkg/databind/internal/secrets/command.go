// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Command struct {
	Path           string   `yaml:"path"`
	Args           []string `yaml:"args,omitempty"`
	PassthroughEnv []string `yaml:"passthrough_environment,omitempty"`
}

type commandGatherer struct {
	cfg *Command
}

// Error handling.
var (
	ErrNoPath                 = errors.New("secrets gatherer command must have a path parameter in order to be executed")
	ErrEmptyResponse          = errors.New("the command returned an empty response")
	ErrInvalidResponse        = errors.New("the command returned an invalid response")
	ErrParseResNoData         = errors.New("missing required field 'data'")
	ErrParseResInvalidData    = errors.New("invalid type for field 'data'")
	ErrParseResTTLInvalidType = errors.New("invalid type for field 'ttl'")
	ErrValidation             = errors.New("validation error")
	ErrCommandRun             = errors.New("failed to run command")
	ErrParseCommandResponse   = errors.New("failed to parse command response")

	ErrTTLNotFound = errors.New("TTL value not found")
	ErrTTLInvalid  = errors.New("TTL value is not valid")
)

func commandExitError(exitErr *exec.ExitError) error {
	return fmt.Errorf("%s: %s", exitErr.Error(), string(exitErr.Stderr))
}

// End error handling.

type cmdResponse struct {
	CmdTTL  string         `json:"ttl,omitempty"`
	CmdData map[string]any `json:"data"`
}

// UnmarshalJSON is the custom unmarshaler for cmdResponse.
// The top-level field "data" is required, but the field "ttl" is optional.
func (c *cmdResponse) UnmarshalJSON(data []byte) error {
	// Top-level field "data" is required
	genericRes := map[string]any{}
	if err := json.Unmarshal(data, &genericRes); err != nil {
		return err
	}

	if _, ok := genericRes["data"]; !ok {
		return fmt.Errorf("%w: %v", ErrParseCommandResponse, ErrParseResNoData)
	}
	// The nested data field must be either a string or a map[string]any.

	d, err := stringOrMapStringAny(genericRes["data"])
	if err != nil {
		return fmt.Errorf("%w: %v", ErrParseCommandResponse, ErrParseResInvalidData)
	}

	c.CmdData = d

	if ttl, ok := genericRes["ttl"]; ok {
		if s, ok := ttl.(string); ok {
			c.CmdTTL = s
		} else {
			return fmt.Errorf("%w: %v", ErrParseCommandResponse, ErrParseResTTLInvalidType)
		}
	}

	return nil
}

func (c *cmdResponse) TTL() (time.Duration, error) {

	if c.CmdTTL == "" {
		return 0, ErrTTLNotFound
	}

	if duration, err := time.ParseDuration(c.CmdTTL); err != nil {
		return 0, ErrTTLInvalid
	} else {
		return duration, nil
	}
}

func (c *cmdResponse) Data() (map[string]any, error) {
	// The nested data field must be either a string or a map[string]any.
	// If it's a string, we return it as a map[string]any with a single key equal to the string value.
	return stringOrMapStringAny(c.CmdData)
}

func stringOrMapStringAny(val any) (map[string]any, error) {
	if m, ok := val.(map[string]any); ok {
		return m, nil
	}

	if s, ok := val.(string); ok {
		// The string value can be referenced as a key.
		return map[string]any{s: s}, nil
	}

	return nil, fmt.Errorf("%w: invalid type: %T", ErrParseCommandResponse, val)
}

func (cmd *Command) Validate() error {
	if cmd.Path == "" {
		return fmt.Errorf("%w: %v", ErrValidation, ErrNoPath)
	}

	return nil
}

// CommandGatherer instantiates a Command variable gatherer from the given configuration. The fetching process
// will return either a map containing access paths to the stored JSON or a byte string value
// E.g. if the stored Secret is `{"account":{"user":"test1","password":"test2"}}`, the returned Map
// contents will be:
// "account.user"     -> "test1"
// "account.password" -> "test2".
// For command return values that include a "data" field and an optional "ttl" field, the access paths
// will consider the "data" field as the root of the JSON.
// E.g. if the stored Secret is `{"data":{"user":"test1","password":"test2"}}`, the returned Map
// contents will be:
// "user"     -> "test1"
// "password" -> "test2".
func CommandGatherer(cmd *Command) func() (any, error) {
	cfg := commandGatherer{cmd}

	return func() (any, error) {
		dt, err := cfg.get()
		if err != nil {
			return nil, err
		}

		return dt, nil
	}
}

func (cg *commandGatherer) get() (any, error) {
	res, err := runCommand(cg.cfg)
	if err != nil {
		return nil, err
	}

	return parsePayload(res)
}

func parsePayload(payload []byte) (any, error) {
	// Parse result as cmdResponse
	var cmdRes *cmdResponse
	if err := json.Unmarshal(payload, &cmdRes); err == nil {
		return cmdRes, nil
	}

	// Parse result as map[string]any
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err == nil {
		return obj, nil
	}

	// Return the string if possible
	str := string(payload)
	if len(str) > 0 {
		return str, nil
	}

	return nil, fmt.Errorf("%w: %v", ErrParseCommandResponse, ErrInvalidResponse)
}

// runCommand executes the given command and returns the contents of `stdout`.
func runCommand(cmd *Command) ([]byte, error) {
	if _, err := exec.LookPath(cmd.Path); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCommandRun, err)
	}

	// Runnign arbitrary commands can be unsafe. Linter will complain
	command := exec.Command(cmd.Path, cmd.Args...) //nolint:gosec
	command.Env = setCmdEnv(cmd.PassthroughEnv)

	res, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {

			return nil, fmt.Errorf("%w: %v", ErrCommandRun, commandExitError(exitErr))
		}
		return nil, fmt.Errorf("%w: %v", ErrCommandRun, err)
	}

	trimmedRes := bytes.TrimSpace(res)
	// If the command output is empty, return an error
	if len(trimmedRes) == 0 {
		return nil, fmt.Errorf("%w: %v", ErrCommandRun, ErrEmptyResponse)
	}

	return trimmedRes, nil
}

// setCmdEnv will clear the environment variables of the given command and set only
// the ones provided in the `passthrough_environment` config.
// `passthrough_environment` can be a list of environment variables or regular expressions.
func setCmdEnv(passthroughEnv []string) []string {
	set := make(map[string]string)
	env := getOSEnv()

	for _, k := range passthroughEnv {
		regex, err := regexp.Compile(k)
		if err != nil {
			if v, ok := os.LookupEnv(k); ok {
				set[k] = v
			}
		} else {
			for k, v := range env {
				if regex.MatchString(k) {
					set[k] = v
				}
			}
		}
	}

	return toEnvVarSlice(set)
}

// getOSEnv returns the current environment variables in a friendlier structure.
func getOSEnv() map[string]string {
	env := make(map[string]string)
	keyValuePairLen := 2

	for _, envVar := range os.Environ() {
		pair := strings.SplitN(envVar, "=", keyValuePairLen)
		if len(pair) != keyValuePairLen {
			continue
		}
		env[pair[0]] = pair[1]
	}

	return env
}

// toEnvVarSlice converts a map of environment variables to a slice of strings in the format `key=value`.
// This is the format expected by the `exec` package's `Cmd.Env` field.
func toEnvVarSlice(env map[string]string) []string {
	res := make([]string, 0, len(env))
	for k, v := range env {
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}

	return res
}
