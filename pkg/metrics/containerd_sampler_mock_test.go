// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package metrics

import (
	"context"
	"errors"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/typeurl/v2"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var errUnimplemented = errors.New("unimplemented")

// Implements ContainerdInterface (pkgs/helpers/containerd_utils.go).
type MockBaseContainerdImpl struct{}

func (m *MockBaseContainerdImpl) Initialize() error {
	return nil
}

func (m *MockBaseContainerdImpl) Namespaces() ([]string, error) {
	return nil, errUnimplemented
}

func (m *MockBaseContainerdImpl) Containers(_ ...string) (map[string][]containerd.Container, error) {
	return nil, errNoContainers
}

// Implements ContainerdInterface (pkgs/helpers/containerd_utils.go).
type MockContainerContainerdImpl struct {
	MockBaseContainerdImpl
}

func (mc *MockContainerContainerdImpl) Initialize() error {
	return nil
}

func (mc *MockContainerContainerdImpl) Namespaces() ([]string, error) {
	return nil, errUnimplemented
}

func (mc *MockContainerContainerdImpl) Containers(_ ...string) (map[string][]containerd.Container, error) {
	// default container with a running task
	container := &MockContainerdContainer{
		id: func() string {
			return containerID
		},
		task: func(_ context.Context, _ cio.Attach) (containerd.Task, error) {
			return &MockContainerdTask{}, nil
		},
	}

	// container without a running task
	noRunningContainer := &MockContainerdContainer{
		id: func() string {
			return containerID2
		},
		task: func(_ context.Context, _ cio.Attach) (containerd.Task, error) {
			return nil, errdefs.ErrNotFound
		},
	}

	return map[string][]containerd.Container{"default": {container, noRunningContainer}}, nil
}

type MockContainerWithDataContainerdImpl struct {
	MockBaseContainerdImpl
}

func (mc *MockContainerWithDataContainerdImpl) Containers(_ ...string) (map[string][]containerd.Container, error) {
	container := MockContainerContainerdImpl{}

	return container.Containers()
}

type MockContainerWithNoPids struct {
	MockBaseContainerdImpl
}

func (mc *MockContainerWithNoPids) Containers(_ ...string) (map[string][]containerd.Container, error) {
	container := &MockContainerdContainerNoPids{} //nolint:exhaustruct

	return map[string][]containerd.Container{"default": {container}}, nil
}

// Mock implementation for containerd.Container interface.
type MockContainerdContainer struct {
	id   func() string
	task func(_ context.Context, _ cio.Attach) (containerd.Task, error)
}

func (m *MockContainerdContainer) ID() string {
	// return default containerID if no custom id function is set
	if m.id == nil {
		return containerID
	}
	return m.id()
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
	return nil, errUnimplemented
}

func (m *MockContainerdContainer) Spec(_ context.Context) (*oci.Spec, error) {
	return nil, errUnimplemented
}

func (m *MockContainerdContainer) Task(ctx context.Context, attach cio.Attach) (containerd.Task, error) { //nolint:ireturn
	// return empty task if no custom task function is set
	if m.task == nil {
		return &MockContainerdTask{}, nil
	}
	return m.task(ctx, attach)
}

func (m *MockContainerdContainer) Image(_ context.Context) (containerd.Image, error) { //nolint:ireturn
	return &MockContainerdImage{}, nil
}

func (m *MockContainerdContainer) Labels(_ context.Context) (map[string]string, error) {
	return map[string]string{
		"label1": "value1",
		"label2": "value2",
	}, nil
}

func (m *MockContainerdContainer) SetLabels(_ context.Context, _ map[string]string) (map[string]string, error) {
	return nil, errUnimplemented
}

func (m *MockContainerdContainer) Extensions(_ context.Context) (map[string]typeurl.Any, error) {
	return nil, errUnimplemented
}

func (m *MockContainerdContainer) Update(_ context.Context, _ ...containerd.UpdateContainerOpts) error {
	return nil
}

func (m *MockContainerdContainer) Checkpoint(_ context.Context, _ string, _ ...containerd.CheckpointOpts) (containerd.Image, error) { //nolint:ireturn
	return nil, errUnimplemented
}

// END: Mock implementation for containerd.Container interface.

type MockContainerdContainerNoPids struct {
	MockContainerdContainer
}

func (m *MockContainerdContainerNoPids) Task(_ context.Context, _ cio.Attach) (containerd.Task, error) { //nolint:ireturn
	return &MockContainerdTaskNoPids{}, nil //nolint:exhaustruct
}

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
	return nil, errUnimplemented
}

func (m *MockContainerdTask) Kill(_ context.Context, _ syscall.Signal, _ ...containerd.KillOpts) error {
	return nil
}

func (m *MockContainerdTask) Wait(_ context.Context) (<-chan containerd.ExitStatus, error) {
	return nil, errUnimplemented
}

func (m *MockContainerdTask) CloseIO(_ context.Context, _ ...containerd.IOCloserOpts) error {
	return nil
}

func (m *MockContainerdTask) Resize(_ context.Context, _, _ uint32) error {
	return nil
}

func (m *MockContainerdTask) IO() cio.IO { //nolint:ireturn
	return nil
}

func (m *MockContainerdTask) Status(_ context.Context) (containerd.Status, error) {
	return containerd.Status{}, nil //nolint:exhaustruct
}

func (m *MockContainerdTask) Pause(_ context.Context) error {
	return nil
}

func (m *MockContainerdTask) Resume(_ context.Context) error {
	return nil
}

func (m *MockContainerdTask) Exec(_ context.Context, _ string, _ *specs.Process, _ cio.Creator) (containerd.Process, error) { //nolint:ireturn
	return nil, errUnimplemented
}

func (m *MockContainerdTask) Pids(_ context.Context) ([]containerd.ProcessInfo, error) {
	return []containerd.ProcessInfo{
		{ //nolint:exhaustruct
			Pid: 123,
		},
	}, nil
}

func (m *MockContainerdTask) Checkpoint(_ context.Context, _ ...containerd.CheckpointTaskOpts) (containerd.Image, error) { //nolint:ireturn
	return nil, errUnimplemented
}

func (m *MockContainerdTask) Update(_ context.Context, _ ...containerd.UpdateTaskOpts) error {
	return nil
}

func (m *MockContainerdTask) LoadProcess(_ context.Context, _ string, _ cio.Attach) (containerd.Process, error) { //nolint:ireturn
	return nil, errUnimplemented
}

func (m *MockContainerdTask) Metrics(_ context.Context) (*types.Metric, error) {
	return nil, errUnimplemented
}

func (m *MockContainerdTask) Spec(_ context.Context) (*oci.Spec, error) {
	return nil, errUnimplemented
}

// END: Mock implementation for containerd.Task interface.

type MockContainerdTaskNoPids struct {
	MockContainerdTask
}

func (m *MockContainerdTaskNoPids) Pids(_ context.Context) ([]containerd.ProcessInfo, error) {
	return nil, errCannotGetPids
}

// Mock implementation for containerd.Image interface.
type MockContainerdImage struct{}

func (m *MockContainerdImage) Name() string {
	return "image1"
}

func (m *MockContainerdImage) Platform() platforms.MatchComparer { //nolint:ireturn
	return nil
}

func (m *MockContainerdImage) Spec(_ context.Context) (ocispec.Image, error) {
	return ocispec.Image{}, nil //nolint:exhaustruct
}

func (m *MockContainerdImage) Target() ocispec.Descriptor {
	return ocispec.Descriptor{
		Digest: digest.Digest("sha256:1234567890"),
	}
}

func (m *MockContainerdImage) Labels() map[string]string {
	return nil
}

func (m *MockContainerdImage) Unpack(_ context.Context, _ string, _ ...containerd.UnpackOpt) error {
	return errUnimplemented
}

func (m *MockContainerdImage) RootFS(_ context.Context) ([]digest.Digest, error) { //nolint:ireturn
	return nil, errUnimplemented
}

func (m *MockContainerdImage) Size(_ context.Context) (int64, error) {
	return 0, errUnimplemented
}

func (m *MockContainerdImage) Usage(_ context.Context, _ ...containerd.UsageOpt) (int64, error) {
	return 0, errUnimplemented
}
func (m *MockContainerdImage) Config(_ context.Context) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{}, errUnimplemented
}

func (m *MockContainerdImage) IsUnpacked(_ context.Context, _ string) (bool, error) {
	return false, errUnimplemented
}

func (m *MockContainerdImage) ContentStore() content.Store {
	return nil
}

func (m *MockContainerdImage) Metadata() images.Image {
	return images.Image{}
}

// END: Mock implementation for containerd.Image interface.
