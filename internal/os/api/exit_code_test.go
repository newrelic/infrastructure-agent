// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitCodeErr_Error(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		expected string
	}{
		{
			name:     "exit code 0",
			exitCode: 0,
			expected: "returned non zero exit: 0",
		},
		{
			name:     "exit code 1",
			exitCode: 1,
			expected: "returned non zero exit: 1",
		},
		{
			name:     "exit code 3 (restart)",
			exitCode: 3,
			expected: "returned non zero exit: 3",
		},
		{
			name:     "negative exit code",
			exitCode: -1,
			expected: "returned non zero exit: -1",
		},
		{
			name:     "large exit code",
			exitCode: 255,
			expected: "returned non zero exit: 255",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ExitCodeErr{exitCode: tt.exitCode}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestExitCodeErr_ExitCode(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{
			name:     "exit code success",
			exitCode: ExitCodeSuccess,
		},
		{
			name:     "exit code error",
			exitCode: ExitCodeError,
		},
		{
			name:     "exit code restart",
			exitCode: ExitCodeRestart,
		},
		{
			name:     "custom exit code",
			exitCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ExitCodeErr{exitCode: tt.exitCode}
			assert.Equal(t, tt.exitCode, err.ExitCode())
		})
	}
}

func TestNewExitCodeErr(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{
			name:     "create with success code",
			exitCode: ExitCodeSuccess,
		},
		{
			name:     "create with error code",
			exitCode: ExitCodeError,
		},
		{
			name:     "create with restart code",
			exitCode: ExitCodeRestart,
		},
		{
			name:     "create with negative code",
			exitCode: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewExitCodeErr(tt.exitCode)
			assert.NotNil(t, err)
			assert.Equal(t, tt.exitCode, err.ExitCode())
		})
	}
}

func TestExitCodeConstants(t *testing.T) {
	assert.Equal(t, 0, ExitCodeSuccess)
	assert.Equal(t, 1, ExitCodeError)
	assert.Equal(t, 3, ExitCodeRestart)
}

func TestCheckExitCode_NilError(t *testing.T) {
	result := CheckExitCode(nil)
	assert.Equal(t, ExitCodeSuccess, result)
}

func TestExitCodeErr_ImplementsError(_ *testing.T) {
	var _ error = (*ExitCodeErr)(nil)
}
