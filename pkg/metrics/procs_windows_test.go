// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows
// +build windows

package metrics

import (
	"context"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"syscall"
	"testing"
	"time"

	"github.com/StackExchange/wmi"
	ffTest "github.com/newrelic/infrastructure-agent/internal/feature_flags/test"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/config"

	"github.com/shirou/gopsutil/v3/cpu"

	"github.com/newrelic/infrastructure-agent/internal/agent"
)

func TestProcessAllowedList(t *testing.T) {
	t.Skip("failing for releases")

	// this test assumes that go is running
	cfg := config.Config{AllowedListProcessSample: []string{"go.exe"}}
	testAgent, err := agent.NewAgent(
		&cfg,
		"1",
		"userAgent",
		ffTest.EmptyFFRetriever)
	assert.NoError(t, err)
	testAgentConfig := testAgent.Context
	pm := NewProcsMonitor(testAgentConfig)
	results, err := pm.Sample()
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func getAllWin32ProcsWMI() ([]win32_Process, error) {
	var dst []win32_Process
	q := wmi.CreateQuery(&dst, "") + " WHERE ProcessID != 0 AND ExecutablePath IS NOT NULL"
	err := wmi.QueryNamespace(q, &dst, config.DefaultWMINamespace)
	if err != nil {
		return []win32_Process{}, fmt.Errorf("could not get win32Procs: %s", err)
	}
	if len(dst) < 1 {
		return []win32_Process{}, fmt.Errorf("could not get win32Proc: empty")
	}
	return dst, nil
}

func TestGetAllProcesses(t *testing.T) {
	exePath := "notepad.exe"
	cmd := exec.Command(exePath)
	cmd.Start()
	defer cmd.Process.Kill()
	creationDate := time.Now()
	time.Sleep(100 * time.Millisecond)

	// test using Win32
	processes, err := getAllWin32Procs(getWin32APIProcessPath, getWin32Proc)()
	assert.NoError(t, err)

	found := false
	for _, proc := range processes {
		if proc.ProcessID == uint32(cmd.Process.Pid) {
			assert.Equal(t, exePath, proc.Name, "Process name doesn't match")
			assert.InDelta(t, float64(creationDate.UnixNano()), float64(proc.CreationDate.UnixNano()), float64(100*time.Millisecond), "Process %s(%d) creation time is not correct", exePath, cmd.Process.Pid)
			found = true
			break
		}
	}
	assert.True(t, found, "Process %s not found!", exePath)

	// test using Win32 WMI
	processes, err = getAllWin32Procs(getWin32APIProcessPath, getWin32ProcFromWMI)()
	assert.NoError(t, err)

	found = false

	for _, proc := range processes {
		if proc.ProcessID == uint32(cmd.Process.Pid) {
			assert.Equal(t, exePath, proc.Name, "Process name doesn't match")
			assert.InDelta(t, float64(creationDate.UnixNano()), float64(proc.CreationDate.UnixNano()), float64(100*time.Millisecond), "Process %s(%d) creation time is not correct", exePath, cmd.Process.Pid)
			found = true

			break
		}
	}

	assert.True(t, found, "Process %s not found!", exePath)

	// test using WMI
	processesWMI, err := getAllWin32ProcsWMI()
	assert.NoError(t, err)

	found = false
	for _, proc := range processesWMI {
		if proc.ProcessID == uint32(cmd.Process.Pid) {
			assert.Equal(t, exePath, proc.Name, "[WMI] Process name doesn't match")
			assert.InDelta(t, float64(creationDate.UnixNano()), float64(proc.CreationDate.UnixNano()), float64(100*time.Millisecond), "[WMI] Process %s(%d) creation time is not correct", exePath, cmd.Process.Pid)
			found = true
			break
		}
	}
	assert.True(t, found, "[WMI] Process %s not found!", exePath)
}

func TestCPUTotal(t *testing.T) {
	cpu := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     1.0,
		GuestNice: 1.0,
		Idle:      1.0,
		Iowait:    1.0,
		Irq:       1.0,
		Nice:      1.0,
		Softirq:   1.0,
		Steal:     1.0,
		System:    1.0,
		User:      1.0,
	}

	assert.InDelta(t, 10.0, cpuTotal(cpu), 0.01)
}

func Test_checkContainerNotRunning(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "match",
			args: args{err: errors.New("Error response from daemon: Container e9c57d578de9e487f6f703d04b1b237b1ff3d926d9cc2a4adfcbe8e1946e841f is not running")},
			want: "e9c57d578de9e487f6f703d04b1b237b1ff3d926d9cc2a4adfcbe8e1946e841f",
		},
		{
			name: "match2",
			args: args{err: errors.New("Error response from daemon: Container cb33a2dfaa4b25dddcd509b434bc6cd6c088a4e39a2611776d45fdb02b763039 is not running")},
			want: "cb33a2dfaa4b25dddcd509b434bc6cd6c088a4e39a2611776d45fdb02b763039",
		},
		{
			name: "nomatch",
			args: args{err: errors.New("not legit")},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containerIDFromNotRunningErr(tt.args.err); got != tt.want {
				t.Errorf("check() = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:paralleltest
func TestProcessSampler_Sample_DisabledDockerDecorator(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := config.NewConfig()
	cfg.ProcessContainerDecoration = false
	ctx.On("Config").Return(cfg)

	// The container sampler getter should not be called
	containerSamplerGetter = func(cacheTTL time.Duration, dockerAPIVersion, dockerContainerdNamespace string) []ContainerSampler {
		t.Errorf("containerSamplerGetter should not be called")

		return nil
	}
	defer func() {
		containerSamplerGetter = GetContainerSamplers
	}()

	var expected []ContainerSampler
	sampler := NewProcsMonitor(ctx)
	assert.Equal(t, expected, sampler.containerSamplers)
}

//nolint:paralleltest
func TestProcessSampler_Sample_DockerDecoratorEnabledByDefault(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := config.NewConfig()
	ctx.On("Config").Return(cfg)

	containerSamplerGetter = func(cacheTTL time.Duration, dockerAPIVersion, dockerContainerdNamespace string) []ContainerSampler {
		return []ContainerSampler{&fakeContainerSampler{}}
	}

	defer func() {
		containerSamplerGetter = GetContainerSamplers
	}()

	expected := []ContainerSampler{&fakeContainerSampler{}}
	sampler := NewProcsMonitor(ctx)
	assert.Equal(t, expected, sampler.containerSamplers)
}

//nolint:paralleltest
func TestProcessSampler_Sample_DockerDecoratorEnabledWithNoConfig(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(nil)

	containerSamplerGetter = func(cacheTTL time.Duration, dockerAPIVersion, dockerContainerdNamespace string) []ContainerSampler {
		return []ContainerSampler{&fakeContainerSampler{}}
	}

	defer func() {
		containerSamplerGetter = GetContainerSamplers
	}()

	expected := []ContainerSampler{&fakeContainerSampler{}}
	sampler := NewProcsMonitor(ctx)
	assert.Equal(t, expected, sampler.containerSamplers)
}

func Benchmark_checkContainerNotRunning(b *testing.B) {
	err := errors.New("Error response from daemon: Container e9c57d578de9e487f6f703d04b1b237b1ff3d926d9cc2a4adfcbe8e1946e841f is not running")
	for i := 0; i < b.N; i++ {
		if id := containerIDFromNotRunningErr(err); id != "e9c57d578de9e487f6f703d04b1b237b1ff3d926d9cc2a4adfcbe8e1946e841f" {
			b.Fatalf("check() = %s, want %s", id, "e9c57d578de9e487f6f703d04b1b237b1ff3d926d9cc2a4adfcbe8e1946e841f")
		}
	}
}

func TestProcess_WithoutPath(t *testing.T) {
	// GIVEN processes running
	exePath := "notepad.exe"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, exePath)
	err := cmd.Start()
	assert.NoErrorf(t, err, "Failed executing s% process", exePath)

	// AND a testing logger
	log.SetOutput(ioutil.Discard) // discard logs to not break race tests
	log.SetLevel(logrus.DebugLevel)
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	// WHEN get all the processes and could no get the Path
	processes, err := getAllWin32Procs(pathProvideError, getWin32Proc)()
	assert.NoError(t, err, "Expected to have obtained all processes without any errors")
	assert.True(t, len(processes) > 1, "Expected to have obtained at least one (notepad) process")

	// THEN should return all failed processes without the location Path
	for _, p := range processes {
		assert.NotNil(t, p.ExecutablePath, "Expected that process executable path not to be nil for process: %v", p.Name)
		expectedExecutablePathValue := ""
		assert.Equal(t, expectedExecutablePathValue, *p.ExecutablePath)
	}

	for _, p := range processes {
		assertLogProcessData(t, p.Name, p.ProcessID, hook.AllEntries())
	}

	le := hook.LastEntry()
	assert.Equal(t, logrus.DebugLevel, le.Level)
	assert.Equal(t, errors.New("error retrieving the process path"), le.Data["error"])
	assert.Equal(t, "Cannot query executable path.", le.Message)
	assert.Equal(t, "Metrics", le.Data["component"])
	assert.Equal(t, "ProcessSampler", le.Data["sampler"])
}

func assertLogProcessData(t *testing.T, name string, id uint32, logEntries []*logrus.Entry) {
	const logNotFound = -1
	i := sort.Search(len(logEntries), func(i int) bool {
		return logEntries[i].Data["name"] == name && logEntries[i].Data["process_id"] == id
	})
	assert.Truef(t, i != logNotFound, "Either the name: %s and/or the process id: %v was not logged it", name, id)
}

func pathProvideError(_ syscall.Handle) (*string, error) {
	return nil, errors.New("error retrieving the process path")
}

type fakeContainerSampler struct{}

func (cs *fakeContainerSampler) Enabled() bool {
	return true
}

func (*fakeContainerSampler) NewDecorator() (ProcessDecorator, error) { //nolint:ireturn
	return &fakeDecorator{}, nil
}

type fakeDecorator struct{}

func (pd *fakeDecorator) Decorate(process *types.ProcessSample) {
	process.ContainerImage = "decorated"
	process.ContainerLabels = map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
}
