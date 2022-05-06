// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"fmt"
	protocol2 "github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/protocol"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/newrelic/infrastructure-agent/internal/agent"
)

var (
	// finds matches of either ${blahblah} or $blahblha (and groups them)
	regex, _ = regexp.Compile(`\$\{(.+?)[}]|\$(.+)`)
)

type PluginRunner struct {
	instances []*PluginInstance
	registry  *PluginRegistry
	closeWait *sync.WaitGroup
	agent     iAgent
}

type iAgent interface {
	RegisterPlugin(agent.Plugin)
	GetContext() agent.AgentContext
}

// ArgumentsToEnvVars returns the environment variables that will be passed to the
// external plugin command. This implies that the plugin arguments are
// passed as environment variables to the integrations.
func ArgumentsToEnvVars(verbose int, arguments map[string]string) map[string]string {
	envVars := make(map[string]string)
	envVars["VERBOSE"] = fmt.Sprintf("%v", verbose)

	// Pass the integration arguments as environment variables to the command
	for k, v := range arguments {
		envVars[strings.ToUpper(k)] = expand(v)
	}
	return envVars
}

func expand(v string) string {
	matches := regex.FindAllStringSubmatch(v, -1)
	// if we have matches, the string we want is either in index 1 or 2 of the match
	for _, ms := range matches {
		if len(ms) > 0 {
			match := ms[1]
			if len(match) <= 0 {
				match = ms[2]
			}
			// if we have an env var value for the match we replace it in the original string
			if val, found := os.LookupEnv(match); found {
				v = strings.Replace(v, ms[0], val, 1)
			}
		}
	}
	// no match, return as-is
	return v
}

// ParsePayload parses a string containing a JSON payload with the format of our
// SDK for v1, v2 and v3 protocols. Protocol v4 is not supported because this function is
// only used by v3 integration format and older.
func ParsePayload(raw []byte, forceV2ToV3Upgrade bool) (dataV3 protocol2.PluginDataV3, protocolVersion int, err error) {
	protocolVersion, err = protocol2.VersionFromPayload(raw, forceV2ToV3Upgrade)
	if err != nil {
		return
	}

	dataV3, err = protocol2.ParsePayload(raw, protocolVersion)

	return
}
