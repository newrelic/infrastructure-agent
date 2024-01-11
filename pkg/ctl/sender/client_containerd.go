// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package sender

import (
	"context"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/namespaces"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/pkg/errors"
)

type ContainerdClient struct {
	client      *containerd.Client
	containerID string
	namespace   string
}

// NewContainerdClient creates a containerised agent control client for Containerd containers.
func NewContainerdClient(namespace, containerID string) (c Client, err error) {
	if containerID == "" {
		err = RequiredContainerIDErr
		return
	}

	if !helpers.IsContainerdRunning() {
		err = helpers.ErrNoContainerd
		return
	}

	cl, err := containerd.New(helpers.UnixContainerdSocket)
	if err != nil {
		err = errors.Wrap(err, "failed to initialize containerd client")
		return
	}

	c = &ContainerdClient{
		client:      cl,
		containerID: containerID,
		namespace:   namespace,
	}

	return
}

// Notify will notify a running agent process inside a docker container.
func (c *ContainerdClient) Notify(ctx context.Context, _ ipc.Message) error {
	ctx = namespaces.WithNamespace(ctx, c.namespace)

	killRequest := tasks.KillRequest{
		ContainerID: c.containerID,
		Signal:      uint32(syscall.SIGUSR1),
	}
	_, err := c.client.TaskService().Kill(ctx, &killRequest)
	return err
}

// GetID returns the identification for the notified agent.
func (c *ContainerdClient) GetID() string {
	return c.containerID
}
