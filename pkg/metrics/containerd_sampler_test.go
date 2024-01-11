// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package metrics

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	metricTypes "github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/stretchr/testify/assert"
)

const containerID = "container1"
const containerID2 = "container2"

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

	assert.EqualError(t, err, "containerd sampler: containerd error: no active containerd instance found")
	assert.Nil(t, containerdClient)
}

func TestContainerdProcessDecoratorNoContainers(t *testing.T) {
	t.Parallel()

	mock := &MockBaseContainerdImpl{}
	pidsCache := newPidsCache(metadataCacheTTL)

	_, err := newContainerdDecorator(mock, pidsCache, config.DefaultDockerContainerdNamespace)
	assert.EqualError(t, err, "containerd sampler: no containers")
}

func TestContainerdProcessDecoratorNoProcessContainers(t *testing.T) {
	t.Parallel()

	mock := &MockContainerWithNoPids{} //nolint:exhaustruct
	pidsCache := newPidsCache(metadataCacheTTL)

	_, err := newContainerdDecorator(mock, pidsCache, config.DefaultDockerContainerdNamespace)
	assert.EqualError(t, err, "containerd sampler: unable to get pids for container")
}

func TestContainerdProcessDecoratorDecorateProcessSampleBadProcessID(t *testing.T) {
	t.Parallel()

	mock := &MockContainerWithDataContainerdImpl{}
	pidsCache := newPidsCache(metadataCacheTTL)

	decorator, err := newContainerdDecorator(mock, pidsCache, config.DefaultDockerContainerdNamespace)
	assert.NoError(t, err)

	process := metricTypes.ProcessSample{ProcessID: 666, ContainerLabels: map[string]string{}} //nolint:exhaustruct
	decorator.Decorate(&process)

	assert.Equal(t, process.ContainerImage, "")
	assert.Equal(t, process.ContainerImageName, "")
	assert.Equal(t, process.ContainerLabels, map[string]string{})
	assert.Equal(t, process.ContainerID, "")
	assert.Equal(t, process.ContainerName, "")
	assert.Equal(t, process.Contained, "")
}

func TestContainerdProcessDecoratorDecorateProcessSample(t *testing.T) {
	t.Parallel()

	mock := &MockContainerWithDataContainerdImpl{}
	pidsCache := newPidsCache(metadataCacheTTL)

	decorator, err := newContainerdDecorator(mock, pidsCache, config.DefaultDockerContainerdNamespace)
	assert.NoError(t, err)

	// ensure container without running state is not cached
	_, found := pidsCache.get(containerID2)
	assert.Equal(t, false, found)

	process := metricTypes.ProcessSample{ProcessID: 123} //nolint:exhaustruct
	decorator.Decorate(&process)

	assert.Equal(t, process.ContainerImage, "sha256:1234567890")
	assert.Equal(t, process.ContainerImageName, "image1")
	assert.Equal(t, process.ContainerLabels, map[string]string{"label1": "value1", "label2": "value2"})
	assert.Equal(t, process.ContainerID, containerID)
	assert.Equal(t, process.ContainerName, containerID)
	assert.Equal(t, process.Contained, "true")
}
