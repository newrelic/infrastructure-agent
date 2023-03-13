// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package v4

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultLoggingBinDir(t *testing.T) {
	agentDir := "C:\\some\\agent\\dir"
	integrationsDir := "integrations_dir"

	testCases := []struct {
		name                  string
		ffEnabled             bool
		ffExists              bool
		expectedLoggingBinDir string
	}{
		{
			name:                  "no ff",
			ffExists:              false,
			expectedLoggingBinDir: "C:\\some\\agent\\dir\\integrations_dir\\logging",
		},
		{
			name:                  "no ff but enabled",
			ffExists:              false,
			ffEnabled:             true,
			expectedLoggingBinDir: "C:\\some\\agent\\dir\\integrations_dir\\logging",
		},
		{
			name:                  "disabled ff",
			ffExists:              true,
			ffEnabled:             false,
			expectedLoggingBinDir: "C:\\some\\agent\\dir\\integrations_dir\\logging",
		},
		{
			name:                  "enabled ff",
			ffExists:              true,
			ffEnabled:             true,
			expectedLoggingBinDir: "C:\\some\\agent\\dir\\integrations_dir\\logging",
		},
	}

	cfg := fBSupervisorConfig{
		agentDir:        agentDir,
		integrationsDir: integrationsDir,
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			loggingBinDir := cfg.defaultLoggingBinDir(tt.ffExists, tt.ffEnabled)
			assert.Equal(t, tt.expectedLoggingBinDir, loggingBinDir)
		})
	}
}

func Test_defaultFluentBitExePath(t *testing.T) {
	agentDir := "C:\\some\\agent\\dir"
	integrationsDir := "integrations_dir"

	testCases := []struct {
		name            string
		ffEnabled       bool
		ffExists        bool
		expectedExePath string
	}{
		{
			name:            "no ff",
			ffExists:        false,
			expectedExePath: "C:\\some\\agent\\dir\\integrations_dir\\logging\\fluent-bit.exe",
		},
		{
			name:            "no ff but enabled",
			ffExists:        false,
			ffEnabled:       true,
			expectedExePath: "C:\\some\\agent\\dir\\integrations_dir\\logging\\fluent-bit.exe",
		},
		{
			name:            "disabled ff",
			ffExists:        true,
			ffEnabled:       false,
			expectedExePath: "C:\\some\\agent\\dir\\integrations_dir\\logging\\fluent-bit.exe",
		},
		{
			name:            "enabled ff",
			ffExists:        true,
			ffEnabled:       true,
			expectedExePath: "C:\\some\\agent\\dir\\integrations_dir\\logging\\fluent-bit.exe",
		},
	}

	cfg := fBSupervisorConfig{
		agentDir:        agentDir,
		integrationsDir: integrationsDir,
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			loggingBinDir := cfg.defaultLoggingBinDir(tt.ffExists, tt.ffEnabled)
			exePath := cfg.defaultFluentBitExePath(tt.ffExists, tt.ffEnabled, loggingBinDir)
			assert.Equal(t, tt.expectedExePath, exePath)
		})
	}
}
