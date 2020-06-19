// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"
	"github.com/sirupsen/logrus"
)

var pslog = log.WithFieldsF(func() logrus.Fields {
	return logrus.Fields{
		"component": "Metrics",
		"sampler":   "ProcessSampler",
	}
})

// We use pointers to floats instead of plain floats so that if we don't set one
// of the values, it will not be sent to Dirac. (Not using pointers would mean
// that Go would always send a default value of 0.)
type ProcessSample struct {
	sample.BaseEvent
	ProcessDisplayName    string   `json:"processDisplayName"`
	ProcessID             int32    `json:"processId"`
	CommandName           string   `json:"commandName"`
	User                  string   `json:"userName,omitempty"`
	MemoryRSSBytes        int64    `json:"memoryResidentSizeBytes"`
	MemoryVMSBytes        int64    `json:"memoryVirtualSizeBytes"`
	CPUPercent            float64  `json:"cpuPercent"`
	CPUUserPercent        float64  `json:"cpuUserPercent"`
	CPUSystemPercent      float64  `json:"cpuSystemPercent"`
	ContainerImage        string   `json:"containerImage,omitempty"`
	ContainerImageName    string   `json:"containerImageName,omitempty"`
	ContainerName         string   `json:"containerName,omitempty"`
	ContainerID           string   `json:"containerId,omitempty"`
	Contained             string   `json:"contained,omitempty"`
	CmdLine               string   `json:"commandLine,omitempty"`
	Status                string   `json:"state,omitempty"`
	ParentProcessID       int32    `json:"parentProcessId,omitempty"`
	ThreadCount           int32    `json:"threadCount,omitempty"`
	FdCount               *int32   `json:"fileDescriptorCount,omitempty"`
	IOReadCountPerSecond  *float64 `json:"ioReadCountPerSecond,omitempty"`
	IOWriteCountPerSecond *float64 `json:"ioWriteCountPerSecond,omitempty"`
	IOReadBytesPerSecond  *float64 `json:"ioReadBytesPerSecond,omitempty"`
	IOWriteBytesPerSecond *float64 `json:"ioWriteBytesPerSecond,omitempty"`
	IOTotalReadCount      *uint64  `json:"ioTotalReadCount,omitempty"`
	IOTotalWriteCount     *uint64  `json:"ioTotalWriteCount,omitempty"`
	IOTotalReadBytes      *uint64  `json:"ioTotalReadBytes,omitempty"`
	IOTotalWriteBytes     *uint64  `json:"ioTotalWriteBytes,omitempty"`
	// Auxiliary values, not to be reported
	LastIOCounters  *process.IOCountersStat `json:"-"`
	ContainerLabels map[string]string       `json:"-"`
}

func NewProcessSample(pid int32) *ProcessSample {
	return &ProcessSample{
		ProcessID: pid,
		Contained: "false",
	}
}

// Deprecated (just for windows compatibility before we migrate it to the "process" package)
type ProcessWrapper interface {
	Ppid() (int32, error)
}

// Deprecated (just for windows compatibility before we migrate it to the "process" package)
type ProcessCacheEntry struct {
	process    ProcessWrapper
	lastCPU    *cpu.TimesStat
	lastSample *ProcessSample // The last event we generated for this process, so we can re-use metadata which doesn't change
}

// Deprecated (just for windows compatibility before we migrate it to the "process" package)
type ProcessInterrogator interface {
	NewProcess(int32) (ProcessWrapper, error)
}

type InternalProcess struct {
	process *process.Process
}

func (pw *InternalProcess) Ppid() (int32, error) {
	return pw.process.Ppid()
}

func NewInternalProcess(p *process.Process) *InternalProcess {
	return &InternalProcess{
		process: p,
	}
}

type InternalProcessInterrogator struct {
	privileged bool
}

func NewInternalProcessInterrogator(privileged bool) *InternalProcessInterrogator {
	return &InternalProcessInterrogator{
		privileged: privileged,
	}
}

func (ip *InternalProcessInterrogator) NewProcess(pid int32) (ProcessWrapper, error) {
	process, err := process.NewProcess(pid)
	return NewInternalProcess(process), err
}
