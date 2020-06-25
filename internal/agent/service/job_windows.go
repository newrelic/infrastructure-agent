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

	// Using unsafe operation we are using the handle inside os.Process.
	processHandle := (*struct {
		pid    int
		handle windows.Handle
	})(unsafe.Pointer(process)).handle

	err := windows.AssignProcessToJobObject(jo.handle, processHandle)
	if err != nil {
		return fmt.Errorf("failed to assign process to job object: %v", err)
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
