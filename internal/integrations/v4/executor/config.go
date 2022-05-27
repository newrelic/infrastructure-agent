// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package executor

import (
	"os"
	"regexp"
	"strings"
)

// Config describes the context to execute a command: user, directory and environment variables.
type Config struct {
	User            string
	Directory       string
	IntegrationName string
	// Manually specified variables
	Environment map[string]string
	// Global variables that need to be retrieved before the integration runs
	Passthrough []string
}

// for testing purposes
var (
	environ   = os.Environ
	lookupEnv = os.LookupEnv
)

// BuildEnv returns the environment configuration of an executable, merging the
// user-defined environment variables from the configuration files with the
// global passthrough_environment configuration.
// For backwards-compatibility reasons, the passthrough has higher precedence
// than the configured Environment
func (c *Config) BuildEnv() map[string]string {
	if len(c.Passthrough) == 0 {
		return c.Environment
	}
	env := map[string]string{}
	// copy the Environment to not pollute the original with the passthrough
	for k, v := range c.Environment {
		env[k] = v
	}
	allEnvVars := environ()
	// override with passthrough, if defined
	for _, k := range c.Passthrough {
		r, err := regexp.Compile(k)
		if err != nil {
			if v, ok := lookupEnv(k); ok {
				env[k] = v
			}
		} else {
			for _, envVar := range allEnvVars {
				pair := strings.SplitN(envVar, "=", 2)
				if len(pair) != 2 {
					continue
				}
				envVar = pair[0]
				if r.MatchString(envVar) {
					if v, ok := lookupEnv(envVar); ok {
						env[envVar] = v
					}
				}
			}
		}
	}
	return env
}

// clones the configuration so we can manually replace ${config.path}
// in different instances
func (c *Config) deepClone() *Config {
	if c == nil {
		return nil
	}
	var envCopy map[string]string
	if c.Environment != nil {
		envCopy = map[string]string{}
		for k, v := range c.Environment {
			envCopy[k] = v
		}
	}
	var passthroughCopy []string
	if c.Passthrough != nil {
		passthroughCopy = make([]string, len(c.Passthrough))
		copy(passthroughCopy, c.Passthrough)
	}
	return &Config{
		User:            c.User,
		Directory:       c.Directory,
		IntegrationName: c.IntegrationName,
		Environment:     envCopy,
		Passthrough:     passthroughCopy,
	}
}
