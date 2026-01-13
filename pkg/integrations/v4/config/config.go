// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/shlex"
)

// ConfigEntry holds an integrations YAML configuration entry. It may define multiple types of tasks
type ConfigEntry struct {
	InstanceName     string            `json:"name" yaml:"name"`                           // integration instance name
	CLIArgs          []string          `json:"cli_args" yaml:"cli_args"`                   // optional when executable is deduced by "name" instead of "exec"
	Exec             ShlexOpt          `json:"exec" yaml:"exec"`                           // it may be a CLI string or a YAML array
	Env              map[string]string `json:"env" yaml:"env"`                             // User-defined environment variables
	Interval         string            `json:"interval" yaml:"interval"`                   // User-defined interval string (duration notation)
	HeartbeatTimeout string            `json:"heartbeat_timeout" yaml:"heartbeat_timeout"` // User-defined timeout string for interation to try until this(duration notation)
	Timeout          *time.Duration    `json:"timeout"           yaml:"timeout"`
	User             string            `json:"integration_user"  yaml:"integration_user"`
	WorkDir          string            `json:"working_dir"       yaml:"working_dir"`
	Labels           map[string]string `json:"labels"            yaml:"labels"`
	Tags             map[string]string `json:"tags"              yaml:"tags"`
	When             EnableConditions  `json:"when"              yaml:"when"`

	// Legacy definition commands
	Command         string            `json:"command"          yaml:"command"`
	Arguments       map[string]string `json:"arguments"        yaml:"arguments"`
	IntegrationName string            `json:"integration_name" yaml:"integration_name"`
	InventorySource string            `json:"inventory_source" yaml:"inventory_source"`

	// Config embeds a configuration file as a string. It can't coexist with TemplatePath
	Config interface{} `json:"config" yaml:"config"`
	// TemplatePath specifies the path of an external configuration file. It can't coexist with Config
	TemplatePath  string `json:"config_template_path" yaml:"config_template_path"`
	LogsQueueSize int    `json:"logs_queue_size"      yaml:"logs_queue_size"`
}

// EnableConditions condition the execution of an integration to the trueness of ALL the conditions
type EnableConditions struct {
	// Feature allows enabling/disabling the OHI via agent cfg "feature" or cmd-channel Feature Flag
	Feature string `yaml:"feature"`
	// FileExists conditions the execution of the OHI only if the given file path exists
	FileExists string `yaml:"file_exists"`
	// EnvExists conditions the execution of the OHI only if the given
	// environment variables exists and match the value.
	EnvExists map[string]string `yaml:"env_exists"`
}

// ShlexOpt is a wrapper around []string so we can use go-shlex for shell tokenizing
type ShlexOpt []string

// Set the value
func (s *ShlexOpt) Set(value string) error {
	valueSlice, err := shlex.Split(value)
	*s = valueSlice
	return err
}

// Type returns the type of the value
func (s *ShlexOpt) Type() string {
	return "command"
}

func (s *ShlexOpt) String() string {
	if len(*s) == 0 {
		return ""
	}
	return fmt.Sprint(*s)
}

func (s *ShlexOpt) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		return s.Set(single)
	} else {
		*s = multi
	}
	return nil
}

// Value returns the value as a string slice
func (s *ShlexOpt) Value() []string {
	return *s
}

// checks that the format is correct and fixes possible nil leaks with default values
func (cf *ConfigEntry) Sanitize() error {
	if cf.InstanceName == "" {
		return errors.New("integration entry requires a non-empty 'name' field")
	}

	if len(cf.Exec) > 0 && len(cf.CLIArgs) > 0 {
		return errors.New("use either 'exec' or 'cli_args' but not both")
	}

	// Checking if there is any configuration file or path to be passed externally to the integration
	if cf.Config != nil && cf.TemplatePath != "" {
		return fmt.Errorf("only 'config' or 'config_template_path' is allowed, not both at the same time")
	}

	// Avoids undefined environment configuration to leak a nil map
	if cf.Env == nil {
		cf.Env = map[string]string{}
	}
	return nil
}

// UppercaseEnvVars transforms all lowercase env vars defined in the config to uppercase
func (cf *ConfigEntry) UppercaseEnvVars() {
	if cf.Env == nil {
		return
	}
	for k, e := range cf.Env {
		upperCasedKey := strings.ToUpper(k)
		if k != upperCasedKey {
			delete(cf.Env, k)
		}
		cf.Env[upperCasedKey] = e
	}
}
