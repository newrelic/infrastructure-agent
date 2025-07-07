// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package executor implements the executor module which is able to execute an individual command and
// forward the standard input, standard error, and go errors by a set of channels
//go:build windows
// +build windows

package executor

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

const (
	aboveNormalPriorityClass = 0x00008000
	belowNormalPriorityClass = 0x00004000
	highPriorityClass        = 0x00000080
	idlePriorityClass        = 0x00000040
	normalPriorityClass      = 0x00000020
	realtimePriorityClass    = 0x00000100
)

var priorityClasses = map[string]uint32{
	"Normal":      normalPriorityClass,
	"Idle":        idlePriorityClass,
	"High":        highPriorityClass,
	"RealTime":    realtimePriorityClass,
	"BelowNormal": belowNormalPriorityClass,
	"AboveNormal": aboveNormalPriorityClass,
}

func Test_startProcess_processShouldInheritParentPriorityClass(t *testing.T) {

	tests := []struct {
		name          string
		priorityClass string
	}{
		{
			name:          "normal priority class",
			priorityClass: "Normal",
		},
		{
			name:          "high priority class",
			priorityClass: "High",
		},
		{
			name:          "above normal priority class",
			priorityClass: "AboveNormal",
		},
	}

	secondsToSleep := "1"
	// GIVEN a runnable instance that points to a working executable
	r := FromCmdSlice(testhelp.Command(fixtures.SleepCmd, secondsToSleep), execConfig(t))
	assert.NotNil(t, r)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AND setting a priority class the current process
			err := windows.SetPriorityClass(windows.CurrentProcess(), priorityClasses[tt.priorityClass])
			assert.Nil(t, err)
			priorityClass, err := windows.GetPriorityClass(windows.CurrentProcess())
			assert.Equal(t, priorityClasses[tt.priorityClass], priorityClass)

			// WHEN we invoke the runnable
			cmd := r.buildCommand(context.Background())
			err = startProcess(cmd)
			assert.Nil(t, err)

			// THEN it should have the same priority class as runnable
			procHndl, err := processHandle(cmd)
			assert.Nil(t, err)
			procPriorityClass, err := windows.GetPriorityClass(procHndl)
			assert.Equal(t, priorityClasses[tt.priorityClass], procPriorityClass)
		})
	}
}
func Test_setPriorityClass(t *testing.T) {
	t.Parallel() // Add parallel execution for the test
	tests := []struct {
		name          string
		priorityClass string
		expectError   bool
		handleClosed  bool
	}{
		{
			name:          "high priority class",
			priorityClass: "High",
			expectError:   false,
			handleClosed:  false,
		},
		{
			name:          "invalid handle error",
			priorityClass: "Normal",
			expectError:   true,
			handleClosed:  false,
		},
		{
			name:          "closed handle error",
			priorityClass: "Normal",
			expectError:   true,
			handleClosed:  true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel() // Add parallel execution for subtests
			// Start a real, short-lived process
			cmd := exec.Command("cmd", "/c", "ping", "127.0.0.1", "-n", "3") // A simple command that runs for a few seconds
			err := cmd.Start()
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

			if testCase.expectError {
				cmd.Process = nil // Simulate invalid or closed handle
			} else {
				// Set priority class for the current process
				err := windows.SetPriorityClass(windows.CurrentProcess(), priorityClasses[testCase.priorityClass])
				require.NoError(t, err)
			}

			// Test setPriorityClass
			err = setPriorityClass(cmd)
			if testCase.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify priority class
				handle, err := processHandle(cmd)
				require.NoError(t, err)
				procPriorityClass, err := windows.GetPriorityClass(handle)
				require.NoError(t, err)
				assert.Equal(t, priorityClasses[testCase.priorityClass], procPriorityClass)
			}
		})
	}
}
