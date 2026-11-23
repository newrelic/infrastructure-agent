// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"bytes"
	"math"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_newHarvester(t *testing.T) {
	tests := []struct {
		name                         string
		cfg                          *config.Config
		expectedPrivileged           bool
		expectedDisableZeroRSSFilter bool
		expectedStripCommandLine     bool
	}{
		{
			name:                         "no config",
			cfg:                          nil,
			expectedPrivileged:           true,
			expectedDisableZeroRSSFilter: false,
			expectedStripCommandLine:     config.DefaultStripCommandLine,
		},
		{
			name:                         "root mode",
			cfg:                          &config.Config{RunMode: config.ModeRoot},
			expectedPrivileged:           true,
			expectedDisableZeroRSSFilter: false,
			expectedStripCommandLine:     false,
		},
		{
			name:                         "privileged mode",
			cfg:                          &config.Config{RunMode: config.ModePrivileged},
			expectedPrivileged:           true,
			expectedDisableZeroRSSFilter: false,
			expectedStripCommandLine:     false,
		},
		{
			name:                         "unprivileged mode",
			cfg:                          &config.Config{RunMode: config.ModeUnprivileged},
			expectedPrivileged:           false,
			expectedDisableZeroRSSFilter: false,
			expectedStripCommandLine:     false,
		},
		{
			name:                         "DisableZeroRSSFilter",
			cfg:                          &config.Config{DisableZeroRSSFilter: true},
			expectedPrivileged:           false,
			expectedDisableZeroRSSFilter: true,
			expectedStripCommandLine:     false,
		},
		{
			name:                         "stripCommandLine",
			cfg:                          &config.Config{StripCommandLine: true},
			expectedPrivileged:           false,
			expectedDisableZeroRSSFilter: false,
			expectedStripCommandLine:     true,
		},
		{
			name:                         "dont stripCommandLine",
			cfg:                          &config.Config{StripCommandLine: false},
			expectedPrivileged:           false,
			expectedDisableZeroRSSFilter: false,
			expectedStripCommandLine:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := new(mocks.AgentContext)
			ctx.On("Config").Once().Return(tt.cfg)
			h := newHarvester(ctx)
			assert.Equal(t, tt.expectedPrivileged, h.privileged)
			assert.Equal(t, tt.expectedDisableZeroRSSFilter, h.disableZeroRSSFilter)
			assert.Equal(t, tt.expectedStripCommandLine, h.stripCommandLine)
			ctx.AssertExpectations(t)
		})
	}
}

func TestDarwinHarvester_populateStaticData_OnErrorOnCmd(t *testing.T) {
	ctx := new(mocks.AgentContext)
	snapshot := &SnapshotMock{}

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)

	h := newHarvester(ctx)
	errorOnCmd := errors.New("this is some error")
	snapshot.ShouldReturnCmdLine(!h.stripCommandLine, "", errorOnCmd)

	sample := &types.ProcessSample{}
	err := h.populateStaticData(sample, snapshot)

	assert.Equal(t, errors.Cause(err), errorOnCmd)
	assert.Equal(t, sample, &types.ProcessSample{})
	mock.AssertExpectationsForObjects(t, ctx, snapshot)
}

func TestDarwinHarvester_populateStaticData_LogOnErrorOnUsername(t *testing.T) {
	ctx := new(mocks.AgentContext)
	snapshot := &SnapshotMock{}
	cmdLine := "some cmd line"
	command := "some command"
	var pid int32 = 3
	var ppid int32 = 3

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)

	//Capture the logs
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetLevel(logrus.DebugLevel)

	h := newHarvester(ctx)
	errorOnUsername := errors.New("this is some username error")
	snapshot.ShouldReturnCmdLine(!h.stripCommandLine, cmdLine, nil)
	snapshot.ShouldReturnUsername("", errorOnUsername)
	snapshot.ShouldReturnPid(pid)
	snapshot.ShouldReturnPpid(ppid)
	snapshot.ShouldReturnCommand(command)

	sample := &types.ProcessSample{}
	err := h.populateStaticData(sample, snapshot)
	assert.Nil(t, err)

	//get log output
	written := output.String()
	assert.Contains(t, written, "Can't get Username for process.")

	assert.Equal(t, sample, &types.ProcessSample{
		CmdLine:         cmdLine,
		CommandName:     command,
		ProcessID:       pid,
		ParentProcessID: ppid,
	})

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx, snapshot)
}

func TestDarwinHarvester_populateStaticData_NoErrorOnUsername(t *testing.T) {
	ctx := new(mocks.AgentContext)
	snapshot := &SnapshotMock{}
	cmdLine := "some cmd line"
	command := "some command"
	username := "some username"
	var pid int32 = 3
	var ppid int32 = 3

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)

	//Capture the logs
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetLevel(logrus.DebugLevel)

	h := newHarvester(ctx)
	snapshot.ShouldReturnCmdLine(!h.stripCommandLine, cmdLine, nil)
	snapshot.ShouldReturnUsername(username, nil)
	snapshot.ShouldReturnPid(pid)
	snapshot.ShouldReturnPpid(ppid)
	snapshot.ShouldReturnCommand(command)

	sample := &types.ProcessSample{}
	err := h.populateStaticData(sample, snapshot)
	assert.Nil(t, err)

	//get log output
	written := output.String()
	assert.Equal(t, written, "")

	assert.Equal(t, sample, &types.ProcessSample{
		CmdLine:         cmdLine,
		CommandName:     command,
		ProcessID:       pid,
		ParentProcessID: ppid,
		User:            username,
	})

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx, snapshot)
}

func TestDarwinHarvester_populateGauges(t *testing.T) {
	ctx := new(mocks.AgentContext)
	snapshot := &SnapshotMock{}

	tests := []struct {
		name           string
		cpuInfo        CPUInfo
		status         string
		threadCount    int32
		vms            int64
		rss            int64
		expectedSample *types.ProcessSample
	}{
		{
			name: "with cpu info",
			cpuInfo: CPUInfo{
				Percent: 45.34,
				User:    21.10,
				System:  24.24,
			},
			status:      "some status",
			threadCount: int32(4),
			vms:         int64(23),
			rss:         int64(34),
			expectedSample: &types.ProcessSample{
				CPUPercent:       45.34,
				CPUUserPercent:   21.10,
				CPUSystemPercent: 24.24,
				Status:           "some status",
				ThreadCount:      int32(4),
				MemoryVMSBytes:   int64(23),
				MemoryRSSBytes:   int64(34),
			},
		},
		{
			name: "no cpu user/system info",
			cpuInfo: CPUInfo{
				Percent: 56.34,
			},
			status:      "some other status",
			threadCount: int32(2),
			vms:         int64(55),
			rss:         int64(66),
			expectedSample: &types.ProcessSample{
				CPUPercent:       56.34,
				CPUUserPercent:   0,
				CPUSystemPercent: 0,
				Status:           "some other status",
				ThreadCount:      int32(2),
				MemoryVMSBytes:   int64(5),
				MemoryRSSBytes:   int64(66),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			cfg := &config.Config{RunMode: config.ModeRoot}
			ctx.On("Config").Once().Return(cfg)

			h := newHarvester(ctx)

			snapshot.ShouldReturnCPUTimes(tt.cpuInfo, nil)
			snapshot.ShouldReturnStatus(tt.status)
			snapshot.ShouldReturnNumThreads(tt.threadCount)
			snapshot.ShouldReturnVmSize(tt.vms)
			snapshot.ShouldReturnVmRSS(tt.rss)

			sample := &types.ProcessSample{}
			err := h.populateGauges(sample, snapshot)
			assert.Nil(t, err)

			assert.Equal(t, tt.cpuInfo.Percent, sample.CPUPercent)
			assert.Equal(t, tt.cpuInfo.User, math.Round(sample.CPUUserPercent*100)/100)
			assert.Equal(t, tt.cpuInfo.System, math.Round(sample.CPUSystemPercent*100)/100)
			assert.Equal(t, tt.status, sample.Status)
			assert.Equal(t, tt.threadCount, sample.ThreadCount)
			assert.Equal(t, tt.vms, sample.MemoryVMSBytes)
			assert.Equal(t, tt.rss, sample.MemoryRSSBytes)

			//mocked objects assertions
			mock.AssertExpectationsForObjects(t, ctx, snapshot)
		})
	}
}

func TestDarwinHarvester_populateGauges_NoCpuInfo(t *testing.T) {
	ctx := new(mocks.AgentContext)
	snapshot := &SnapshotMock{}

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)

	h := newHarvester(ctx)

	cpuInfoErr := errors.New("this is an error")
	snapshot.ShouldReturnCPUTimes(CPUInfo{}, cpuInfoErr)

	sample := &types.ProcessSample{}
	err := h.populateGauges(sample, snapshot)
	assert.Equal(t, cpuInfoErr, err)
	assert.Equal(t, sample, &types.ProcessSample{})

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx, snapshot)
}

func TestDarwinHarvester_determineProcessDisplayName_OnProcessIdInfoAvailable(t *testing.T) {
	ctx := new(mocks.AgentContext)
	commandName := "some command name"
	processName := "some process name"
	processId := 10
	ok := true

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)
	ctx.On("GetServiceForPid", processId).Once().Return(processName, ok)

	//Capture the logs
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetLevel(logrus.DebugLevel)

	h := newHarvester(ctx)

	sample := &types.ProcessSample{CommandName: commandName, ProcessID: int32(processId)}
	name := h.determineProcessDisplayName(sample)

	assert.Equal(t, processName, name)

	//get log output
	written := output.String()
	assert.Contains(t, written, "Using service name as display name.")

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx)
}

func TestDarwinHarvester_determineProcessDisplayName_OnProcessIdInfoNotAvailable(t *testing.T) {
	ctx := new(mocks.AgentContext)
	commandName := "some command name"
	processName := ""
	processId := 10
	ok := false

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)
	ctx.On("GetServiceForPid", processId).Once().Return(processName, ok)

	h := newHarvester(ctx)

	sample := &types.ProcessSample{CommandName: commandName, ProcessID: int32(processId)}
	name := h.determineProcessDisplayName(sample)

	assert.Equal(t, commandName, name)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx)
}

func TestDarwinHarvester_Do_DontReportIfMemoryZero(t *testing.T) {
	ctx := new(mocks.AgentContext)
	processId := int32(1)
	elapsedSeconds := 12.0 //not used right now

	cfg := &config.Config{RunMode: config.ModeRoot, DisableZeroRSSFilter: false}
	ctx.On("Config").Once().Return(cfg)

	proc := &ProcessMock{}

	proc.ShouldReturnName("some name", nil)
	proc.ShouldReturnProcessIdMultipleTimes(processId, 2)
	proc.ShouldReturnNumThreads(3, nil)
	proc.ShouldReturnStatus([]string{"some status"}, nil)
	proc.ShouldReturnMemoryInfo(
		&process.MemoryInfoStat{
			RSS: 0,
			VMS: 0,
		}, nil)
	proc.ShouldReturnCPUPercent(34.45, nil)
	proc.ShouldReturnTimes(&cpu.TimesStat{User: 34, System: 0.45}, nil)
	proc.ShouldReturnUsername("some username", nil)

	h := newHarvester(ctx)
	h.processRetriever = func(int32) (Process, error) {
		return proc, nil
	}

	var expectedSample *types.ProcessSample
	sample, err := h.Do(processId, elapsedSeconds)
	assert.Error(t, err, "process with zero rss")
	assert.Equal(t, expectedSample, sample)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx)
}

func TestDarwinHarvester_Do_NoError(t *testing.T) {
	ctx := new(mocks.AgentContext)
	processName := "some process name"
	processId := int32(1)
	elapsedSeconds := 12.0 //not used right now
	ok := false

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)
	ctx.On("GetServiceForPid", int(processId)).Once().Return(processName, ok)

	proc := &ProcessMock{}

	proc.ShouldReturnName("some name", nil)
	proc.ShouldReturnProcessIdMultipleTimes(processId, 2)
	proc.ShouldReturnNumThreads(3, nil)
	proc.ShouldReturnStatus([]string{"some status"}, nil)
	proc.ShouldReturnMemoryInfo(
		&process.MemoryInfoStat{
			RSS: 45,
			VMS: 22,
		}, nil)
	proc.ShouldReturnCPUPercent(34.45, nil)
	proc.ShouldReturnTimes(&cpu.TimesStat{User: 34, System: 0.45}, nil)
	proc.ShouldReturnUsername("some username", nil)
	proc.ShouldReturnCmdLine("a command", nil)

	h := newHarvester(ctx)
	h.processRetriever = func(int32) (Process, error) {
		return proc, nil
	}

	sample, err := h.Do(processId, elapsedSeconds)

	assert.Nil(t, err)
	assert.Equal(t, "some name", sample.ProcessDisplayName)
	assert.Equal(t, processId, sample.ProcessID)
	assert.Equal(t, "some name", sample.CommandName)
	assert.Equal(t, "some username", sample.User)
	assert.Equal(t, int64(45), sample.MemoryRSSBytes)
	assert.Equal(t, int64(22), sample.MemoryVMSBytes)
	assert.Equal(t, 34.45, sample.CPUPercent)
	assert.Equal(t, 34.0, math.Round(sample.CPUUserPercent*100)/100)
	assert.Equal(t, 0.45, math.Round(sample.CPUSystemPercent*100)/100)
	assert.Equal(t, "a command", sample.CmdLine)
	assert.Equal(t, "some status", sample.Status)
	assert.Equal(t, int32(0), sample.ParentProcessID)
	assert.Equal(t, int32(3), sample.ThreadCount)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx)
}

func TestDarwinHarvester_Do_ErrorOnProcError(t *testing.T) {
	ctx := new(mocks.AgentContext)
	processId := int32(1)
	elapsedSeconds := 12.0 //not used right now
	var expectedSample *types.ProcessSample

	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Once().Return(cfg)

	procErr := errors.New("some error message")
	h := newHarvester(ctx)
	h.processRetriever = func(int32) (Process, error) {
		return nil, procErr
	}

	sample, err := h.Do(processId, elapsedSeconds)
	assert.Equal(t, errors.Cause(err), procErr)
	assert.Equal(t, expectedSample, sample)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, ctx)
}
