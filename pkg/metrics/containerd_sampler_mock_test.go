// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package metrics

import (
	"context"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Implements ContainerdInterface (pkgs/helpers/containerd_utils.go)
type MockBaseContainerdImpl struct{}

func (m *MockBaseContainerdImpl) Initialize() error {
	return nil
}

func (m *MockBaseContainerdImpl) Namespaces() ([]string, error) {
	return nil, nil
}

func (m *MockBaseContainerdImpl) Containers(_ ...string) (map[string][]containerd.Container, error) {
	return nil, errNoContainers
}

func (m *MockBaseContainerdImpl) ContainerProcesses(containerID string) ([]containerd.ProcessInfo, error) {
	return nil, errContainerDoesNotExist(containerID)
}

// Implements ContainerdInterface (pkgs/helpers/containerd_utils.go)
type MockContainerContainerdImpl struct{}

func (mc *MockContainerContainerdImpl) Initialize() error {
	return nil
}

func (mc *MockContainerContainerdImpl) Namespaces() ([]string, error) {
	return nil, nil
}

func (mc *MockContainerContainerdImpl) Containers(_ ...string) (map[string][]containerd.Container, error) {
	container := &MockContainerdContainer{}

	return map[string][]containerd.Container{"default": {container}}, nil
}

func (mc *MockContainerContainerdImpl) ContainerProcesses(containerID string) ([]containerd.ProcessInfo, error) {
	// container := MockContainerContainerdImpl{}
	return nil, nil
}

type MockContainerWithDataContainerdImpl struct{}

func (mc *MockContainerWithDataContainerdImpl) Initialize() error {
	return nil
}

func (mc *MockContainerWithDataContainerdImpl) Namespaces() ([]string, error) {
	return nil, nil
}

func (mc *MockContainerWithDataContainerdImpl) Containers(_ ...string) (map[string][]containerd.Container, error) {
	container := MockContainerContainerdImpl{}

	return container.Containers()
}

func (mc *MockContainerWithDataContainerdImpl) ContainerProcesses(cID string) ([]containerd.ProcessInfo, error) {
	if cID != containerID {
		return nil, errContainerNotFound
	}

	processes := []containerd.ProcessInfo{{Pid: 123}}

	return processes, nil
}

// Mock implementation for containerd.Container interface.
type MockContainerdContainer struct{}

func (m *MockContainerdContainer) ID() string {
	return containerID
}

func (m *MockContainerdContainer) Info(_ context.Context, _ ...containerd.InfoOpts) (containers.Container, error) {
	return containers.Container{ //nolint:exhaustruct
		ID: containerID,
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		Image: "ubuntu:14.04",
	}, nil
}

func (m *MockContainerdContainer) Delete(_ context.Context, _ ...containerd.DeleteOpts) error {
	return nil
}

func (m *MockContainerdContainer) NewTask(_ context.Context, _ cio.Creator, _ ...containerd.NewTaskOpts) (containerd.Task, error) { //nolint:ireturn
	return nil, nil
}

func (m *MockContainerdContainer) Spec(_ context.Context) (*oci.Spec, error) {
	return nil, nil //nolint:nilnil
}

func (m *MockContainerdContainer) Task(_ context.Context, _ cio.Attach) (containerd.Task, error) { //nolint:ireturn
	return &MockContainerdTask{}, nil
}

func (m *MockContainerdContainer) Image(_ context.Context) (containerd.Image, error) { //nolint:ireturn
	return nil, nil
}

func (m *MockContainerdContainer) Labels(_ context.Context) (map[string]string, error) {
	return map[string]string{
		"label1": "value1",
		"label2": "value2",
	}, nil
}

func (m *MockContainerdContainer) SetLabels(_ context.Context, _ map[string]string) (map[string]string, error) {
	return nil, nil //nolint:nilnil
}

func (m *MockContainerdContainer) Extensions(_ context.Context) (map[string]prototypes.Any, error) {
	return nil, nil //nolint:nilnil
}

func (m *MockContainerdContainer) Update(_ context.Context, _ ...containerd.UpdateContainerOpts) error {
	return nil
}

func (m *MockContainerdContainer) Checkpoint(_ context.Context, _ string, _ ...containerd.CheckpointOpts) (containerd.Image, error) { //nolint:ireturn
	return nil, nil
}

// END: Mock implementation for containerd.Container interface.

// Mock implementation for containerd.Task interface.
type MockContainerdTask struct{}

func (m *MockContainerdTask) ID() string {
	return "task1"
}

func (m *MockContainerdTask) Pid() uint32 {
	return 123
}

func (m *MockContainerdTask) Start(_ context.Context) error {
	return nil
}

func (m *MockContainerdTask) Delete(_ context.Context, _ ...containerd.ProcessDeleteOpts) (*containerd.ExitStatus, error) {
	return nil, nil
}

func (m *MockContainerdTask) Kill(_ context.Context, _ syscall.Signal, _ ...containerd.KillOpts) error {
	return nil
}

func (m *MockContainerdTask) Wait(_ context.Context) (<-chan containerd.ExitStatus, error) {
	return nil, nil
}

func (m *MockContainerdTask) CloseIO(_ context.Context, _ ...containerd.IOCloserOpts) error {
	return nil
}

func (m *MockContainerdTask) Resize(_ context.Context, _, _ uint32) error {
	return nil
}

func (m *MockContainerdTask) IO() cio.IO {
	return nil
}

func (m *MockContainerdTask) Status(_ context.Context) (containerd.Status, error) {
	return containerd.Status{}, nil
}

func (m *MockContainerdTask) Pause(_ context.Context) error {
	return nil
}

func (m *MockContainerdTask) Resume(_ context.Context) error {
	return nil
}

func (m *MockContainerdTask) Exec(_ context.Context, _ string, _ *specs.Process, _ cio.Creator) (containerd.Process, error) {
	return nil, nil
}

func (m *MockContainerdTask) Pids(_ context.Context) ([]containerd.ProcessInfo, error) {
	return nil, ErrCannotGetPids
}

func (m *MockContainerdTask) Checkpoint(_ context.Context, _ ...containerd.CheckpointTaskOpts) (containerd.Image, error) {
	return nil, nil
}

func (m *MockContainerdTask) Update(_ context.Context, _ ...containerd.UpdateTaskOpts) error {
	return nil
}

func (m *MockContainerdTask) LoadProcess(_ context.Context, _ string, _ cio.Attach) (containerd.Process, error) {
	return nil, nil
}

func (m *MockContainerdTask) Metrics(_ context.Context) (*types.Metric, error) {
	return nil, nil
}

func (m *MockContainerdTask) Spec(_ context.Context) (*oci.Spec, error) {
	return nil, nil
}

// END: Mock implementation for containerd.Task interface.
