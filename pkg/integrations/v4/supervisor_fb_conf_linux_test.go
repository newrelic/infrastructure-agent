// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package v4

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_defaultLoggingBinDir(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		ffEnabled      bool
		ffExists       bool
		expectedBinDir string
	}{
		{
			name:           "no ff",
			ffExists:       false,
			expectedBinDir: "/opt/fluent-bit/bin",
		},
		{
			name:           "no ff but enabled",
			ffExists:       false,
			ffEnabled:      true,
			expectedBinDir: "/opt/fluent-bit/bin",
		},
		{
			name:           "disabled ff",
			ffExists:       true,
			ffEnabled:      false,
			expectedBinDir: "/opt/fluent-bit/bin",
		},
		{
			name:           "enabled ff",
			ffExists:       true,
			ffEnabled:      true,
			expectedBinDir: "/opt/td-agent-bit/bin",
		},
	}

	cfg := fBSupervisorConfig{}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			binDir := cfg.defaultLoggingBinDir(testCase.ffExists, testCase.ffEnabled)
			assert.Equal(t, testCase.expectedBinDir, binDir)
		})
	}
}

func Test_defaultFluentBitExePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		ffEnabled       bool
		ffExists        bool
		expectedExePath string
	}{
		{
			name:            "no ff",
			ffExists:        false,
			expectedExePath: "/opt/fluent-bit/bin/fluent-bit",
		},
		{
			name:            "no ff but enabled",
			ffExists:        false,
			ffEnabled:       true,
			expectedExePath: "/opt/fluent-bit/bin/fluent-bit",
		},
		{
			name:            "disabled ff",
			ffExists:        true,
			ffEnabled:       false,
			expectedExePath: "/opt/fluent-bit/bin/fluent-bit",
		},
		{
			name:            "enabled ff",
			ffExists:        true,
			ffEnabled:       true,
			expectedExePath: "/opt/td-agent-bit/bin/td-agent-bit",
		},
	}

	cfg := fBSupervisorConfig{}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			loggingBinDir := cfg.defaultLoggingBinDir(testCase.ffExists, testCase.ffEnabled)
			exePath := cfg.defaultFluentBitExePath(testCase.ffExists, testCase.ffEnabled, loggingBinDir)
			assert.Equal(t, testCase.expectedExePath, exePath)
		})
	}
}
