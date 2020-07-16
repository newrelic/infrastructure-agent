// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	metricTypes "github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const metadataCacheSecs = 30 * time.Second

func TestDocker(t *testing.T) {
	docker := NewDockerSamplerWithClient(&MockContainerDocker{}, metadataCacheSecs, "1.24")
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{DisableZeroRSSFilter: true})
	ctx.On("GetServiceForPid", mock.Anything).Return("", false)

	procs := NewProcsMonitor(ctx)
	procs.containerSampler = docker
	procs.getAllProcs = mockGetAllWin32Procs
	procs.getMemoryInfo = mockGetMemoryInfo
	procs.getStatus = mockGetStatus
	procs.getUsername = mockGetProcessUsername
	procs.getTimes = mockGetProcessTimes
	procs.getCommandLine = mockGetCommandLine
	procs.processInterrogator = &MockProcessInterrogator{}

	samples, err := procs.Sample()
	assert.NoError(t, err)

	assert.Len(t, samples, 3)
	for i, sample := range samples {
		procSample, ok := sample.(*metricTypes.ProcessSample)
		assert.True(t, ok)

		switch i {
		case 0:
			assert.Equal(t, procSample.CommandName, "Notepad.exe")
			assert.EqualValues(t, procSample.ProcessID, 1024)
			assert.Equal(t, procSample.Contained, "false")
			assert.Equal(t, procSample.ContainerImage, "")
			assert.Equal(t, procSample.ContainerImageName, "")
			assert.Equal(t, procSample.ContainerID, "")
			assert.Equal(t, procSample.ContainerName, "")
		case 1:
			assert.Equal(t, procSample.CommandName, "NotepadContainer.exe")
			assert.EqualValues(t, procSample.ProcessID, 123)
		case 2:
			assert.Equal(t, procSample.CommandName, "FakeContainer.exe")
			assert.EqualValues(t, procSample.ProcessID, 456)
		}

		if i == 1 || i == 2 {
			assert.Equal(t, procSample.Contained, "true")
			assert.Equal(t, procSample.ContainerImage, "14.04")
			assert.Equal(t, procSample.ContainerImageName, "ubuntu1")
			assert.Equal(t, procSample.ContainerLabels, map[string]string{"label1": "value1", "label2": "value2"})
			assert.Equal(t, procSample.ContainerID, "cca35d9d")
			assert.Equal(t, procSample.ContainerName, "container1")
		}
	}
}

// Function mocks

func mockGetAllWin32Procs() ([]process.Win32_Process, error) {
	processes := []process.Win32_Process{
		{
			Name:      "Notepad.exe",
			ProcessID: 1024,
		},
		{
			Name:      "NotepadContainer.exe",
			ProcessID: 123,
		},
		{
			Name:      "FakeContainer.exe",
			ProcessID: 456,
		},
	}

	return processes, nil
}

func mockGetMemoryInfo(pid int32) (*MemoryInfoStat, error) {
	return &MemoryInfoStat{}, nil
}

func mockGetStatus(pid int32) (string, error) {
	return "running", nil
}

func mockGetProcessUsername(pid int32) (string, error) {
	return "administrator", nil
}

func mockGetProcessTimes(pid int32) (*SystemTimes, error) {
	return &SystemTimes{}, nil
}

func mockGetCommandLine(pid uint32) (string, error) {
	return "", nil
}

// Docker container mock

type MockContainerDocker struct {
}

func (mc *MockContainerDocker) Initialize(_ string) error {
	return nil
}

func (mc *MockContainerDocker) Containers() ([]types.Container, error) {
	container := types.Container{
		ID:      "cca35d9d",
		ImageID: "ubuntu:14.04",
		Names:   []string{"/container1"},
		Image:   "ubuntu1",
		State:   "Running",
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Command: "/bin/bash arg1 arg2 -password secret",
	}
	return []types.Container{container}, nil
}

func (mc *MockContainerDocker) ContainerTop(containerID string) (titles []string, processes [][]string, err error) {
	if containerID != "cca35d9d" {
		return nil, nil, fmt.Errorf("container not found")
	}

	titles = []string{"Name", "PID", "CPU", "Private Working Set"}
	processes = [][]string{{"NotepadContainer.exe", "123", "00:00:00.437", "598kB"}, {"FakeContainer.exe", "456", "00:00:00.987", "700kB"}}
	return titles, processes, nil
}

// Process mock

type MockInternalProcess struct {
	process *process.Process
}

func (pw *MockInternalProcess) Pid() int32 {
	return pw.process.Pid
}

func (pw *MockInternalProcess) Command() (string, error) {
	return "", nil
}

func (pw *MockInternalProcess) Username() (string, error) {
	return "", nil
}

func (pw *MockInternalProcess) Cmdline() (string, error) {
	return "", nil
}

func (pw *MockInternalProcess) ExecutablePath() (string, error) {
	return "", nil
}

func (pw *MockInternalProcess) Ppid() (int32, error) {
	return 0, nil
}

func (pw *MockInternalProcess) MemoryInfo() (*process.MemoryInfoStat, error) {
	return &process.MemoryInfoStat{}, nil
}

func (pw *MockInternalProcess) Parent() (ProcessWrapper, error) {
	return &MockInternalProcess{}, nil
}

func (pw *MockInternalProcess) IOCounters() (*process.IOCountersStat, error) {
	return &process.IOCountersStat{}, nil
}

func (pw *MockInternalProcess) NumFDs() (int32, error) {
	return 0, nil
}

func (pw *MockInternalProcess) NetIOCounters(pernic bool) ([]net.IOCountersStat, error) {
	return []net.IOCountersStat{}, nil
}

func (pw *MockInternalProcess) NumThreads() (int32, error) {
	return 0, nil
}

func (pw *MockInternalProcess) Status() (string, error) {
	return "", nil
}

func (pw *MockInternalProcess) CPUPercent(time.Duration) (float64, error) {
	return 0, nil
}

func (pw *MockInternalProcess) CPUTimes() (*cpu.TimesStat, error) {
	return &cpu.TimesStat{}, nil
}

// Process interrogator mock

type MockProcessInterrogator struct {
}

func (m *MockProcessInterrogator) Pids() ([]int32, error) {
	return []int32{123}, nil
}

func (m *MockProcessInterrogator) NewProcess(pid int32) (ProcessWrapper, error) {
	p := &process.Process{Pid: pid}
	return &MockInternalProcess{p}, nil
}

func (m *MockProcessInterrogator) FillFromStatus(*metricTypes.ProcessSample) error {
	return nil
}
