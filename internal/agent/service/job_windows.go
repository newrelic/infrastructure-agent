// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// JobObject on Windows allows groups of processes to be managed as a unit.
type JobObject struct {
	handle windows.Handle
}

// NewJob creates a new instance of a Windows Job Object.
func NewJob() (*JobObject, error) {
	jobObjectHandle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create job object: %v", err)
	}

	jobInfo := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	_, err = windows.SetInformationJobObject(
		jobObjectHandle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&jobInfo)),
		uint32(unsafe.Sizeof(jobInfo)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set job object info: %v", err)
	}
	return &JobObject{
		handle: jobObjectHandle,
	}, nil
}

// AddProcess to the JobObject.
func (jo *JobObject) AddProcess(process *os.Process) error {
	// Do not read the handle out of os.Process internals: its memory layout changes between Go
	// versions (it broke in Go 1.23) and AssignProcessToJobObject then fails with an invalid
	// handle. Open our own handle by PID with the access rights the assignment requires.
	const access = windows.PROCESS_SET_QUOTA | windows.PROCESS_TERMINATE

	processHandle, err := windows.OpenProcess(access, false, uint32(process.Pid)) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to open process %d: %w", process.Pid, err)
	}

	defer func() { _ = windows.CloseHandle(processHandle) }()

	err = windows.AssignProcessToJobObject(jo.handle, processHandle)
	if err != nil {
		return fmt.Errorf("failed to assign process to job object: %w", err)
	}

	return nil
}

// Close will terminate the JobObject and also stop all the subprocesses.
func (jo *JobObject) Close() error {
	err := windows.CloseHandle(jo.handle)
	if err != nil {
		return fmt.Errorf("failed to close the job object: %v", err)
	}
	return nil
}
