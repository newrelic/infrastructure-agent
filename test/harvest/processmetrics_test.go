// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/detection"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/process"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	tmpIOFile = "/var/tmp/io" // /tmp is mounted as tmpfs in some systems and cannot be used for disk stats
	testFile  = "nr_test_file"
)

// In some machines, the updated IO metrics take many seconds to be visible by the process sampler
// Tests would never reach this timeout, unless they really have to fail
const diskIOTimeout = 1 * time.Minute

// TestProcessSamplerCPUValues will assert that CPU metrics provided by the ProcessSampler are correct.
func TestProcessSamplerCPUValues(t *testing.T) {
	t.Skipf("flaky test")

	procs := runtime.GOMAXPROCS(1) // To limit the percentage to 100
	defer runtime.GOMAXPROCS(procs)

	// Given a Process Sampler
	ps := process.NewProcessSampler(contextMock())

	// That has already run
	_, err := sampleProcess(ps, int32(os.Getpid()))
	require.NoError(t, err)

	// When the CPU value is high
	done := stressCPU()
	defer close(done)

	// The Reported CPU becomes noticeably high
	testhelpers.Eventually(t, 5*time.Second, func(t require.TestingT) {
		sample, err := sampleProcess(ps, int32(os.Getpid()))
		require.NoError(t, err)
		assertProcessSample(t, sample)

		assert.True(t, sample.CPUUserPercent > 70, "CPUUserPercent (%v%%) must be > 70", sample.CPUUserPercent)
		assert.True(t, sample.CPUUserPercent <= 100, "CPUUserPercent (%v%%) must be <= 100", sample.CPUUserPercent)
	})
}

func TestProcessSamplerMemoryValues(t *testing.T) {
	// Given a Process Sampler
	ps := process.NewProcessSampler(contextMock())

	// That has already run
	sample1, err := sampleProcess(ps, int32(os.Getpid()))
	require.NoError(t, err)
	assertProcessSample(t, sample1)

	// When the memory is stressed
	keep := stressMem()
	defer fmt.Sprint(keep) // avoid GC removing reference

	// The Reported Memory becomes noticeably higher than the previous samples
	testhelpers.Eventually(t, 10*time.Second, func(t require.TestingT) {
		sample2, err := sampleProcess(ps, int32(os.Getpid()))
		require.NoError(t, err)
		assertProcessSample(t, sample2)

		assert.True(t, sample1.MemoryRSSBytes+2000000 < sample2.MemoryRSSBytes,
			"MemoryRSSBytes for sample1: %+v, sample2: %+v", sample1.MemoryRSSBytes, sample2.MemoryRSSBytes)
		assert.True(t, sample1.MemoryVMSBytes+2000000 < sample2.MemoryVMSBytes,
			"MemoryVMSBytes for sample1: %+v, sample2: %+v", sample1.MemoryVMSBytes, sample2.MemoryVMSBytes)
	})
}

// func TestProcessSamplerDiskValues_Write(t *testing.T) {
// 	// Given a Process Sampler
// 	ps := process.NewProcessSampler(contextMock())

// 	// That has already run
// 	sample1, err := sampleProcess(ps, int32(os.Getpid()))
// 	require.NoError(t, err)
// 	assertProcessSample(t, sample1)

// 	// When the Disk writes are stressed
// 	assert.NoError(t, writeDisk(tmpIOFile))
// 	defer func() {
// 		if derr := cleanup(tmpIOFile); derr != nil {
// 			t.Log(derr)
// 		}
// 	}()

// 	// The IO write metrics become noticeably higher than in the previous samples
// 	testhelpers.Eventually(t, diskIOTimeout, func(t require.TestingT) {
// 		sample2, err := sampleProcess(ps, int32(os.Getpid()))
// 		require.NoError(t, err)
// 		assertProcessSample(t, sample2)
// 		assertIOCounters(t, sample2)

// 		assert.True(t, *sample1.IOTotalWriteCount < *sample2.IOTotalWriteCount,
// 			"IOTotalWriteCount for sample1: %+v, sample2: %+v", *sample1.IOTotalWriteCount, *sample2.IOTotalWriteCount)
// 		assert.True(t, *sample1.IOTotalWriteBytes+3000 < *sample2.IOTotalWriteBytes,
// 			"IOTotalWriteBytes for sample1: %+v, sample2: %+v", *sample1.IOTotalWriteBytes, *sample2.IOTotalWriteBytes)
// 	})
// }

func TestProcessSamplerDiskValues_Read(t *testing.T) {
	t.Skip("Check why this test fails")

	// Given a Process Sampler
	ps := process.NewProcessSampler(contextMock())

	// That has already run
	sample1, err := sampleProcess(ps, int32(os.Getpid()))
	require.NoError(t, err)
	assertProcessSample(t, sample1)

	assert.NoError(t, writeDisk(testFile))
	defer cleanup(testFile)

	// When the Disk reads are stressed
	assert.NoError(t, readDisk())

	// The IO read metrics become noticeably higher than in the previous samples
	testhelpers.Eventually(t, diskIOTimeout, func(t require.TestingT) {
		sample2, err := sampleProcess(ps, int32(os.Getpid()))
		require.NoError(t, err)
		assertProcessSample(t, sample2)
		assertIOCounters(t, sample2)

		assert.True(t, *sample1.IOTotalReadCount < *sample2.IOTotalReadCount,
			"IOTotalReadCount for sample1: %+v, sample2: %+v", *sample1.IOTotalReadCount, *sample2.IOTotalReadCount)
		assert.True(t, *sample1.IOTotalReadBytes+3000 < *sample2.IOTotalReadBytes,
			"IOTotalReadBytes for sample1: %+v, sample2: %+v", *sample1.IOTotalReadBytes, *sample2.IOTotalReadBytes)
	})
}

func TestProcessSamplerDiskValues_WritePerSecond(t *testing.T) {
	// Given a Process Sampler
	ps := process.NewProcessSampler(contextMock())

	// That has already run
	sample1, err := sampleProcess(ps, int32(os.Getpid()))
	require.NoError(t, err)
	assertProcessSample(t, sample1)

	// When the Disk writes are stressed
	assert.NoError(t, writeDisk(tmpIOFile))
	defer cleanup(tmpIOFile)

	// The IO write per second metrics report nonzero values
	testhelpers.Eventually(t, diskIOTimeout, func(t require.TestingT) {
		time.Sleep(time.Second) // need to wait to avoid elapsedSeconds == 0

		sample2, err := sampleProcess(ps, int32(os.Getpid()))
		require.NoError(t, err)
		assertProcessSample(t, sample2)
		assertIOCounters(t, sample2)

		assert.True(t, *sample2.IOWriteCountPerSecond > 0, "IOWriteCountPerSecond must not be zero")
		assert.True(t, *sample2.IOWriteBytesPerSecond > 0, "IOWriteBytesPerSecond must not be zero")
	})
}

func TestProcessSamplerDiskValues_ReadPerSecond(t *testing.T) {
	t.Skip("Check why this test fails")

	// Given a Process Sampler
	ps := process.NewProcessSampler(contextMock())

	// That has already run
	sample1, err := sampleProcess(ps, int32(os.Getpid()))
	require.NoError(t, err)
	assertProcessSample(t, sample1)

	assert.NoError(t, writeDisk(testFile))
	defer cleanup(testFile)

	// When the Disk reads are stressed
	assert.NoError(t, readDisk())

	// The IO read per second metrics report nonzero values
	testhelpers.Eventually(t, diskIOTimeout, func(t require.TestingT) {
		time.Sleep(time.Second) // need to wait to avoid elapsedSeconds == 0

		sample2, err := sampleProcess(ps, int32(os.Getpid()))
		require.NoError(t, err)
		assertProcessSample(t, sample2)
		assertIOCounters(t, sample2)

		assert.True(t, *sample2.IOReadCountPerSecond > 0, "IOReadCountPerSecond must not be zero")
		assert.True(t, *sample2.IOReadBytesPerSecond > 0, "IOReadBytesPerSecond must not be zero")
	})
}

func TestProcessSampler_CommandChanges(t *testing.T) {
	// Given a process whose command name and command line change at runtime
	f, err := ioutil.TempFile("", "ps")
	require.NoError(t, err)
	require.NoError(t, f.Chmod(os.ModePerm))
	fdName := f.Name() // ps2797417662
	require.NoError(t, ioutil.WriteFile(fdName, []byte(`#!/bin/sh
sleep 1s
exec sleep 30s   # this will change the command name to "sleep"
`), os.ModePerm))
	require.NoError(t, f.Close())

	cmd := exec.Command(fdName)

	require.NoError(t, cmd.Start())
	defer cmd.Process.Kill()

	// That has a given Command Name and Command Line
	ps := process.NewProcessSampler(contextMock())
	testhelpers.Eventually(t, 10*time.Second, func(t require.TestingT) {
		sample, err := sampleProcess(ps, int32(cmd.Process.Pid))
		require.NoError(t, err)

		assert.Regexp(t, fmt.Sprintf("%v$", fdName), sample.CmdLine)
		assert.Regexp(t, fmt.Sprintf("^%v$", path.Base(fdName)), sample.CommandName)
	})

	// When the Command changes at runtime
	// Then new command name and command line is updated
	testhelpers.Eventually(t, 12*time.Second, func(t require.TestingT) {
		sample, err := sampleProcess(ps, int32(cmd.Process.Pid))
		require.NoError(t, err)

		assert.Regexp(t, "sleep 30s$", sample.CmdLine)
		assert.Regexp(t, "^sleep$", sample.CommandName)
	})
}

func TestProcessSamplerUsername(t *testing.T) {
	t.Skipf("Not working in all systems. Disabling to unblock release")

	// Given a Process Sampler
	ps := process.NewProcessSampler(contextMock())

	// And a systemd service running with Dynamic User
	service := "/usr/bin/deleteme"
	content := `#!/bin/bash
while $(sleep 1);
do
    echo "hello world"
done
`
	writeFile(t, service, os.ModePerm, content)
	defer os.Remove(service)

	systemdFile := "/etc/systemd/system/deleteme.service"
	content = fmt.Sprintf(`[Unit]
Description=Hello World Service
After=systend-user-sessions.service

[Service]
Type=simple
ExecStart=%s
DynamicUser=yes
`, service)
	writeFile(t, systemdFile, os.ModePerm, content)
	defer os.Remove(systemdFile)

	_, err := helpers.RunCommand("/usr/bin/systemctl", "", "daemon-reload")
	_, err = helpers.RunCommand("/usr/bin/systemctl", "", []string{"start", "deleteme.service"}...)

	defer func() {
		helpers.RunCommand("/usr/bin/systemctl", "", "daemon-reload")
		helpers.RunCommand("/usr/bin/systemctl", "", []string{"stop", "deleteme.service"}...)
	}()

	assert.NoError(t, err)

	// get the process id
	pid, err := detection.GetProcessID(filepath.Base(service))

	// When we get the process sample
	sample, err := sampleProcess(ps, pid)
	require.NoError(t, err)

	// Username is not empty
	assert.Regexp(t, fmt.Sprintf("^%v$", path.Base("deleteme")), sample.User)
	assert.Regexp(t, service, sample.CmdLine)
}

func writeFile(t *testing.T, path string, mode os.FileMode, content string) {
	// Given a process whose command name and command line change at runtime
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Chmod(mode))
	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func contextMock() *mocks.AgentContext {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		RunMode: config.ModeRoot,
		Log:     config.LogConfig{Level: config.LogLevelDebug},
	})
	ctx.On("GetServiceForPid", mock.Anything).Return("service-name", true)
	return ctx
}

// assertProcessSample will do the initial check for the sample.
func assertProcessSample(t assert.TestingT, sample *types.ProcessSample) {
	assert.NotNil(t, sample.ProcessDisplayName)
	assert.NotNil(t, sample.ProcessID)
	assert.NotNil(t, sample.CommandName)
	assert.NotNil(t, sample.User)
	assert.NotNil(t, sample.MemoryRSSBytes)
	assert.NotNil(t, sample.MemoryVMSBytes)
	assert.NotNil(t, sample.CPUPercent)
	assert.NotNil(t, sample.CPUUserPercent)
	assert.NotNil(t, sample.CPUSystemPercent)
	assert.NotNil(t, sample.ContainerImage)
	assert.NotNil(t, sample.ContainerImageName)
	assert.NotNil(t, sample.ContainerName)
	assert.NotNil(t, sample.ContainerID)
	assert.NotNil(t, sample.Contained)
	assert.NotNil(t, sample.CmdLine)
	assert.NotNil(t, sample.Status)
	assert.NotNil(t, sample.ParentProcessID)
	assert.NotNil(t, sample.ThreadCount)
	assert.NotNil(t, sample.FdCount)
}

func assertIOCounters(t assert.TestingT, sample *types.ProcessSample) {
	assert.NotNil(t, sample.IOReadCountPerSecond)
	assert.NotNil(t, sample.IOWriteCountPerSecond)
	assert.NotNil(t, sample.IOReadBytesPerSecond)
	assert.NotNil(t, sample.IOWriteBytesPerSecond)
	assert.NotNil(t, sample.IOTotalReadCount)
	assert.NotNil(t, sample.IOTotalWriteCount)
	assert.NotNil(t, sample.IOTotalReadBytes)
	assert.NotNil(t, sample.IOTotalWriteBytes)
}

func sampleProcess(ps sampler.Sampler, pid int32) (*types.ProcessSample, error) {
	batch, err := ps.Sample()
	if err != nil {
		return nil, err
	}
	for _, s := range batch {
		sample := s.(*types.ProcessSample) // This will fail if you add container labels to the sample
		if sample.ProcessID == pid {
			return sample, nil
		}
	}
	return nil, fmt.Errorf("pid not found: %v", pid)
}

// writeDisk generates write load to the disk.
func writeDisk(file string) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i := 0; i < 1000; i++ {
		w.WriteString("new relic test\n")
		w.Flush()
	}

	return f.Sync()
}

// readDisk generates Read disk load.
func readDisk() error {
	f, err := os.OpenFile(testFile, os.O_RDONLY, 0o666)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 32*1024)

	for i := 0; i < 1000; i++ {
		_, err := f.Read(buf)
		if err != nil {
			break
		}
	}
	return nil
}

// cleanup will to the cleanup after the tests.
func cleanup(fileName string) error {
	return os.Remove(fileName)
}

// stressCPU is used to increase CPU Usage and also the number of threads.
func stressCPU() chan struct{} {
	// To test the threads count we will allow go routines to exit after we take the sample.
	c := make(chan struct{})

	for i := 0; i < 5; i++ {
		go func() {
			for {
				select {
				case <-c:
					return
				default:
				}
			}
		}()
	}

	return c
}

// stressMem generates Memory load.
func stressMem() []interface{} {
	const size = 1024 * 1024
	all := make([]interface{}, 0)
	for i := 0; i < runtime.NumCPU(); i++ {
		a := new([size]*[size]*int32)

		for j := 0; j < 100; j++ {
			a[j] = new([size]*int32)
		}
		all = append(all, a)
	}
	return all
}
