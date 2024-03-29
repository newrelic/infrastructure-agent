// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"github.com/shirou/gopsutil/v3/process"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

// ProcessSample data type storing all the data harvested for a process.
// Pointers are used as nil values represent no data.
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

// FlatProcessSample stores the process sampling information as a map
type FlatProcessSample map[string]interface{}

var _ sample.Event = &FlatProcessSample{} // FlatProcessSample implements sample.Event

func (f *FlatProcessSample) Type(eventType string) {
	(*f)["eventType"] = eventType
}

func (f *FlatProcessSample) Entity(key entity.Key) {
	(*f)["entityKey"] = key
}

func (f *FlatProcessSample) Timestamp(timestamp int64) {
	(*f)["timestamp"] = timestamp
}
