// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package executor implements the executor module which is able to execute an individual command and
// forward the standard input, standard error, and go errors by a set of channels
//go:build windows
// +build windows

package executor

import (
	"context"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/stretchr/testify/assert"
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
