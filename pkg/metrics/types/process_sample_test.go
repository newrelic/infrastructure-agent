// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"encoding/json"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlatProcessSample_Type(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
	}{
		{
			name:      "set process sample type",
			eventType: "ProcessSample",
		},
		{
			name:      "empty event type",
			eventType: "",
		},
		{
			name:      "custom event type",
			eventType: "CustomProcess",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fps := &FlatProcessSample{}
			fps.Type(tt.eventType)
			assert.Equal(t, tt.eventType, (*fps)["eventType"])
		})
	}
}

func TestFlatProcessSample_Entity(t *testing.T) {
	tests := []struct {
		name      string
		entityKey entity.Key
	}{
		{
			name:      "standard entity key",
			entityKey: entity.Key("host:my-hostname"),
		},
		{
			name:      "empty entity key",
			entityKey: entity.Key(""),
		},
		{
			name:      "container entity key",
			entityKey: entity.Key("container:abc123"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fps := &FlatProcessSample{}
			fps.Entity(tt.entityKey)
			assert.Equal(t, tt.entityKey, (*fps)["entityKey"])
		})
	}
}

func TestFlatProcessSample_Timestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
	}{
		{
			name:      "positive timestamp",
			timestamp: 1609459200000,
		},
		{
			name:      "zero timestamp",
			timestamp: 0,
		},
		{
			name:      "negative timestamp",
			timestamp: -1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fps := &FlatProcessSample{}
			fps.Timestamp(tt.timestamp)
			assert.Equal(t, tt.timestamp, (*fps)["timestamp"])
		})
	}
}

func TestFlatProcessSample_ImplementsEventInterface(_ *testing.T) {
	var _ interface {
		Type(eventType string)
		Entity(key entity.Key)
		Timestamp(timestamp int64)
	} = &FlatProcessSample{}
}

func TestProcessSample_Fields(t *testing.T) {
	fdCount := int32(100)
	ioReadCount := float64(1000)

	sample := ProcessSample{ //nolint:exhaustruct
		ProcessDisplayName:   "nginx",
		ProcessID:            1234,
		CommandName:          "nginx",
		User:                 "root",
		MemoryRSSBytes:       1024000,
		MemoryVMSBytes:       2048000,
		CPUPercent:           5.5,
		CPUUserPercent:       3.0,
		CPUSystemPercent:     2.5,
		ContainerImage:       "nginx:latest",
		ContainerImageName:   "nginx",
		ContainerName:        "my-nginx",
		ContainerID:          "abc123def456",
		Contained:            "true",
		CmdLine:              "nginx -g daemon off;",
		Status:               "running",
		ParentProcessID:      1,
		ThreadCount:          4,
		FdCount:              &fdCount,
		IOReadCountPerSecond: &ioReadCount,
	}

	assert.Equal(t, "nginx", sample.ProcessDisplayName)
	assert.Equal(t, int32(1234), sample.ProcessID)
	assert.Equal(t, "nginx", sample.CommandName)
	assert.Equal(t, "root", sample.User)
	assert.Equal(t, int64(1024000), sample.MemoryRSSBytes)
	assert.Equal(t, int64(2048000), sample.MemoryVMSBytes)
	assert.InEpsilon(t, 5.5, sample.CPUPercent, 0.001)
	assert.InEpsilon(t, 3.0, sample.CPUUserPercent, 0.001)
	assert.InEpsilon(t, 2.5, sample.CPUSystemPercent, 0.001)
	assert.Equal(t, "nginx:latest", sample.ContainerImage)
	assert.Equal(t, "nginx", sample.ContainerImageName)
	assert.Equal(t, "my-nginx", sample.ContainerName)
	assert.Equal(t, "abc123def456", sample.ContainerID)
	assert.Equal(t, "true", sample.Contained)
	assert.Equal(t, "nginx -g daemon off;", sample.CmdLine)
	assert.Equal(t, "running", sample.Status)
	assert.Equal(t, int32(1), sample.ParentProcessID)
	assert.Equal(t, int32(4), sample.ThreadCount)
	assert.Equal(t, int32(100), *sample.FdCount)
	assert.InEpsilon(t, float64(1000), *sample.IOReadCountPerSecond, 0.001)
}

func TestProcessSample_JSONMarshaling(t *testing.T) {
	sample := ProcessSample{ //nolint:exhaustruct
		ProcessDisplayName: "test-process",
		ProcessID:          42,
		CPUPercent:         10.5,
	}

	data, err := json.Marshal(sample)
	require.NoError(t, err)

	var result map[string]any

	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "test-process", result["processDisplayName"])
	assert.InEpsilon(t, float64(42), result["processId"], 0.001)
	assert.InEpsilon(t, 10.5, result["cpuPercent"], 0.001)
}

func TestProcessSample_OmitEmptyFields(t *testing.T) {
	sample := ProcessSample{ //nolint:exhaustruct
		ProcessDisplayName: "test",
		ProcessID:          1,
	}

	data, err := json.Marshal(sample)
	require.NoError(t, err)

	var result map[string]any

	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Fields with omitempty should not be present when empty
	_, hasContainerID := result["containerId"]
	assert.False(t, hasContainerID)

	_, hasUser := result["userName"]
	assert.False(t, hasUser)
}

func TestFlatProcessSample_MapBehavior(t *testing.T) {
	fps := FlatProcessSample{}

	fps["customField"] = "customValue"
	fps["intField"] = 42

	assert.Equal(t, "customValue", fps["customField"])
	assert.Equal(t, 42, fps["intField"])
	assert.Nil(t, fps["nonexistent"])
}
