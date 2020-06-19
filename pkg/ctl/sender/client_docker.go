// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sender

import (
	"context"

	"github.com/newrelic/infrastructure-agent/pkg/ipc"

	"github.com/docker/docker/client"
	"github.com/newrelic/infrastructure-agent/internal/os/api/signals"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/pkg/errors"
)

type dockerClient struct {
	client      *client.Client
	apiVersion  string
	containerID string
}

// RequiredContainerIDErr is the error to return when container ID is not provided.
var RequiredContainerIDErr = errors.New("container ID of the agent is required")

// NewContainerisedClient creates a containerised agent control client for Docker containers.
// Didn't use helpers.DockerClient because it'll broad the interface breaking SRP.
func NewContainerisedClient(apiVersion string, containerID string) (c Client, err error) {
	if containerID == "" {
		err = RequiredContainerIDErr
		return
	}

	if !helpers.IsDockerRunning() {
		err = helpers.ErrNoDockerd
		return
	}

	cl, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		err = errors.Wrap(err, "failed to initialize docker client")
		return
	}

	c = &dockerClient{
		client:      cl,
		apiVersion:  apiVersion,
		containerID: containerID,
	}

	return
}

// Notify will notify a running agent process inside a docker container.
func (c *dockerClient) Notify(ctx context.Context, _ ipc.Message) (err error) {
	return c.client.ContainerKill(ctx, c.containerID, signals.NotificationStr)
}

// Return the identification for the notified agent.
func (c *dockerClient) GetID() string {
	return c.containerID
}
