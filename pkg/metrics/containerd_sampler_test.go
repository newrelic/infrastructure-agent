// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

const containerID = "cca35d9d"

func TestInitializeContainerdClient(t *testing.T) {
	t.Parallel()

	if !helpers.IsContainerdRunning() {
		t.Skip("Containerd daemon not running")
	}

	containerdClient, err := initializeContainerdClient()

	assert.NoError(t, err)
	assert.NotNil(t, containerdClient)
}

func TestInitializeContainerdClientWithoutContainerd(t *testing.T) {
	t.Parallel()

	if helpers.IsContainerdRunning() {
		t.Skip("Containerd daemon running")
	}

	containerdClient, err := initializeContainerdClient()

	assert.EqualError(t, err, "containerd sampler error: containerd error: no active containerd instance found")
	assert.Nil(t, containerdClient)
}

func TestContainerdProcessDecoratorNoContainers(t *testing.T) {
	t.Parallel()

	mock := &MockBaseContainerdImpl{}
	pidsCache := newPidsCache(metadataCacheTTL)

	_, err := newContainerdDecorator(mock, pidsCache)
	assert.EqualError(t, err, "containerd sampler error: no containers")
}

func TestContainerdProcessDecoratorNoProcessContainers(t *testing.T) {
	t.Parallel()

	mock := &MockContainerContainerdImpl{}
	pidsCache := newPidsCache(metadataCacheTTL)

	_, err := newContainerdDecorator(mock, pidsCache)
	assert.EqualError(t, err, "containerd sampler error: Unable to get pids for container")
}

// func TestContainerdProcessDecoratorDecorateProcessSampleBadProcessID(t *testing.T) {
// 	t.Parallel()

// 	mock := &MockContainerWithDataContainerdImpl{}
// 	pidsCache := newPidsCache(metadataCacheTTL)

// 	decorator, err := newContainerdDecorator(mock, pidsCache)
// 	assert.NoError(t, err)

// 	process := metricTypes.ProcessSample{ProcessID: 666, ContainerLabels: map[string]string{}} //nolint:exhaustruct
// 	decorator.Decorate(&process)

// 	assert.Equal(t, process.ContainerImage, "")
// 	assert.Equal(t, process.ContainerImageName, "")
// 	assert.Equal(t, process.ContainerLabels, map[string]string{})
// 	assert.Equal(t, process.ContainerID, "")
// 	assert.Equal(t, process.ContainerName, "")
// 	assert.Equal(t, process.Contained, "")
// }

// func TestContainerdProcessDecoratorDecorateProcessSample(t *testing.T) {
// 	t.Parallel()

// 	mock := &MockContainerWithDataContainerdImpl{}
// 	pidsCache := newPidsCache(metadataCacheTTL)

// 	decorator, err := newContainerdDecorator(mock, pidsCache)
// 	assert.NoError(t, err)

// 	process := metricTypes.ProcessSample{ProcessID: 123} //nolint:exhaustruct
// 	decorator.Decorate(&process)

// 	assert.Equal(t, process.ContainerImage, "14.04")
// 	assert.Equal(t, process.ContainerImageName, "ubuntu1")
// 	assert.Equal(t, process.ContainerLabels, map[string]string{"label1": "value1", "label2": "value2"})
// 	assert.Equal(t, process.ContainerID, containerID)
// 	assert.Equal(t, process.ContainerName, "container1")
// 	assert.Equal(t, process.Contained, "true")
// }
