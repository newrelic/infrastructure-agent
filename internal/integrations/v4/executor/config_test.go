// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func fakeEnviron() []string {
	return []string{
		"SHELL=/bin/bash",
		"SUDO_GID=1000",
		"SUDO_COMMAND=/usr/bin/su",
		"SUDO_USER=vagrant",
		"PWD=/srv",
		"HOME=/root",
		"USER=root",
		"SUDO_UID=1000",
		"DBUS_SYSTEM_BUS_ADDRESS=unix:path=/var/run/dbus/system_bus_socket",
	}
}

func fakeLookupEnv(k string) (string, bool) {
	data := map[string]string{
		"SHELL":                   "/bin/bash",
		"SUDO_GID":                "1000",
		"SUDO_COMMAND":            "/usr/bin/su",
		"SUDO_USER":               "vagrant",
		"PWD":                     "/srv",
		"HOME":                    "/root",
		"USER":                    "root",
		"SUDO_UID":                "1000",
		"DBUS_SYSTEM_BUS_ADDRESS": "unix:path=/var/run/dbus/system_bus_socket",
	}

	if v, ok := data[k]; ok {
		return v, true
	}
	return "", false
}

func clearOsFunctions() {
	environ = os.Environ
	lookupEnv = os.LookupEnv
}

func TestConfig_BuildEnv(t *testing.T) {
	defer clearOsFunctions()
	environ = fakeEnviron
	lookupEnv = fakeLookupEnv

	tests := []struct {
		name        string
		passthrough []string
		environment map[string]string
		expected    map[string]string
	}{
		{
			name:        "no passthrough",
			passthrough: nil,
			environment: map[string]string{
				"test": "value",
			},
			expected: map[string]string{
				"test": "value",
			},
		},
		{
			name: "no variable found",
			passthrough: []string{
				"BLABLA",
			},
			environment: map[string]string{
				"test": "value",
			},
			expected: map[string]string{
				"test": "value",
			},
		},
		{
			name: "given variable",
			passthrough: []string{
				"SHELL",
			},
			environment: map[string]string{
				"test": "value",
			},
			expected: map[string]string{
				"test":  "value",
				"SHELL": "/bin/bash",
			},
		},
		{
			name: "wildcard",
			passthrough: []string{
				"SUDO.*",
			},
			environment: map[string]string{
				"test": "value",
			},
			expected: map[string]string{
				"test":         "value",
				"SUDO_GID":     "1000",
				"SUDO_COMMAND": "/usr/bin/su",
				"SUDO_USER":    "vagrant",
				"SUDO_UID":     "1000",
			},
		},
		{
			name: "all variables",
			passthrough: []string{
				".*",
			},
			environment: map[string]string{
				"test": "value",
			},
			expected: map[string]string{
				"test":                    "value",
				"SHELL":                   "/bin/bash",
				"SUDO_GID":                "1000",
				"SUDO_COMMAND":            "/usr/bin/su",
				"SUDO_USER":               "vagrant",
				"PWD":                     "/srv",
				"HOME":                    "/root",
				"USER":                    "root",
				"SUDO_UID":                "1000",
				"DBUS_SYSTEM_BUS_ADDRESS": "unix:path=/var/run/dbus/system_bus_socket",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a working runnable that is configured with CLI arguments and env vars AND passthrough env variables
			cfg := Config{}
			cfg.Environment = tt.environment
			cfg.Passthrough = tt.passthrough

			result := cfg.BuildEnv()
			assert.Equal(t, tt.expected, result)
		})
	}
}
