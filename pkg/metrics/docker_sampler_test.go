// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

const metadataCacheTTL = 30 * time.Second

func TestInitializeDockerClient(t *testing.T) {
	if !helpers.IsDockerRunning() {
		t.Skip("Docker daemon not running")
	}

	dockerClient, err := initializeDockerClient("1.24")

	assert.NoError(t, err)
	assert.NotNil(t, dockerClient)
}

func TestInitializeDockerClientWithoutDocker(t *testing.T) {
	if helpers.IsDockerRunning() {
		t.Skip("Docker daemon running")
	}

	dockerClient, err := initializeDockerClient("1.24")

	assert.Equal(t, helpers.ErrNoDockerd, err)
	assert.Nil(t, dockerClient)
}

func TestProcessDecoratorNoContainers(t *testing.T) {
	mock := &MockBaseDocker{}
	pidsCache := newPidsCache(metadataCacheTTL)

	_, err := newDecoratorImpl(mock, pidsCache)
	assert.EqualError(t, err, "no containers")
}

func TestProcessDecoratorNoTopContainers(t *testing.T) {
	mock := &MockContainerDocker{}
	pidsCache := newPidsCache(metadataCacheTTL)

	_, err := newDecoratorImpl(mock, pidsCache)
	assert.EqualError(t, err, "container \"cca35d9d\" does not exist")
}

func TestProcessDecoratorWrongTitles(t *testing.T) {
	mock := &MockContainerWithDataDockerWrongTitles{}
	pidsCache := newPidsCache(metadataCacheTTL)

	_, err := newDecoratorImpl(mock, pidsCache)
	assert.EqualError(t, err, "no PID title found for container \"cca35d9d\" top. Returned titles: [Name CPU Private Working Set]")
}

func TestProcessDecoratorDecorateProcessSampleBadProcessID(t *testing.T) {
	mock := &MockContainerWithDataDocker{}
	pidsCache := newPidsCache(metadataCacheTTL)

	decorator, err := newDecoratorImpl(mock, pidsCache)
	assert.NoError(t, err)

	process := ProcessSample{ProcessID: 666, ContainerLabels: map[string]string{}}
	decorator.Decorate(&process)

	assert.Equal(t, process.ContainerImage, "")
	assert.Equal(t, process.ContainerImageName, "")
	assert.Equal(t, process.ContainerLabels, map[string]string{})
	assert.Equal(t, process.ContainerID, "")
	assert.Equal(t, process.ContainerName, "")
	assert.Equal(t, process.Contained, "")
}

func TestProcessDecoratorDecorateProcessSample(t *testing.T) {
	mock := &MockContainerWithDataDocker{}
	pidsCache := newPidsCache(metadataCacheTTL)

	decorator, err := newDecoratorImpl(mock, pidsCache)
	assert.NoError(t, err)

	process := ProcessSample{ProcessID: 123}
	decorator.Decorate(&process)

	assert.Equal(t, process.ContainerImage, "14.04")
	assert.Equal(t, process.ContainerImageName, "ubuntu1")
	assert.Equal(t, process.ContainerLabels, map[string]string{"label1": "value1", "label2": "value2"})
	assert.Equal(t, process.ContainerID, "cca35d9d")
	assert.Equal(t, process.ContainerName, "container1")
	assert.Equal(t, process.Contained, "true")
}

func TestPidsCacheNoContainer(t *testing.T) {
	pidsCache := newPidsCache(0)

	_, exists := pidsCache.get("FakeContainer")
	assert.False(t, exists)
}

func TestPidsCacheContainerExpired(t *testing.T) {
	pidsCache := newPidsCache(100 * time.Millisecond)
	pidsCache.put("container1", []int32{1, 2, 3, 5, 8, 13})
	time.Sleep(101 * time.Millisecond)

	_, exists := pidsCache.get("container1")
	assert.False(t, exists)
}

func TestPidsCacheContainerExists(t *testing.T) {
	pidsCache := newPidsCache(1 * time.Second)
	pidsCache.put("container1", []int32{1, 2, 3, 5, 8, 13})

	pids, exists := pidsCache.get("container1")
	assert.True(t, exists)
	assert.Equal(t, pids, []int32{1, 2, 3, 5, 8, 13})
}

type MockBaseDocker struct {
}

func (m *MockBaseDocker) Initialize(_ string) error {
	return nil
}

func (m *MockBaseDocker) Containers() ([]types.Container, error) {
	return nil, fmt.Errorf("no containers")
}

func (m *MockBaseDocker) ContainerTop(containerID string) (titles []string, processes [][]string, err error) {
	return nil, nil, fmt.Errorf("container \"%v\" does not exist", containerID)
}

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
	}
	return []types.Container{container}, nil
}

func (mc *MockContainerDocker) ContainerTop(containerID string) (titles []string, processes [][]string, err error) {
	container := MockBaseDocker{}
	return container.ContainerTop(containerID)
}

type MockContainerWithDataDocker struct {
}

func (mc *MockContainerWithDataDocker) Initialize(_ string) error {
	return nil
}

func (mc *MockContainerWithDataDocker) Containers() ([]types.Container, error) {
	container := MockContainerDocker{}
	return container.Containers()
}

func (mc *MockContainerWithDataDocker) ContainerTop(containerID string) (titles []string, processes [][]string, err error) {
	if containerID != "cca35d9d" {
		return nil, nil, fmt.Errorf("container not found")
	}

	titles = []string{"Name", "PID", "CPU", "Private Working Set"}
	processes = [][]string{{"/container1", "123", "00:00:00.437", "598kB"}}
	return titles, processes, nil
}

type MockContainerWithDataDockerWrongTitles struct {
}

func (mc *MockContainerWithDataDockerWrongTitles) Initialize(_ string) error {
	return nil
}

func (mc *MockContainerWithDataDockerWrongTitles) Containers() ([]types.Container, error) {
	container := MockContainerWithDataDocker{}
	return container.Containers()
}

func (mc *MockContainerWithDataDockerWrongTitles) ContainerTop(containerID string) (titles []string, processes [][]string, err error) {
	if containerID != "cca35d9d" {
		return nil, nil, fmt.Errorf("container not found")
	}

	titles = []string{"Name", "CPU", "Private Working Set"}
	processes = [][]string{{"/container1", "00:00:00.437", "598kB"}}
	return titles, processes, nil
}
