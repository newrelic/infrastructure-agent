// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"reflect"
	"testing"
)

func TestConfigEntry_UppercaseEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    ConfigEntry
		expected ConfigEntry
	}{
		{
			name:     "Given lowerCased env vars should get upperCased",
			input:    ConfigEntry{Env: map[string]string{"host": "a-host", "port": ":9999"}},
			expected: ConfigEntry{Env: map[string]string{"HOST": "a-host", "PORT": ":9999"}},
		},
		{
			name:     "Given lowerCased and upperCased env vars should get upperCased",
			input:    ConfigEntry{Env: map[string]string{"host": "a-host", "PORT": ":9999"}},
			expected: ConfigEntry{Env: map[string]string{"HOST": "a-host", "PORT": ":9999"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.UppercaseEnvVars()
			if !reflect.DeepEqual(tt.input.Env, tt.expected.Env) {
				t.Errorf("Expected: %v, got: %v", tt.expected, tt.input)
			}
		})
	}
}
