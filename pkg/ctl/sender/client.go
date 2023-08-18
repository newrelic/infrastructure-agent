// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sender

import (
	"context"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/detection"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	RuntimeDocker     = "docker"
	RuntimeContainerd = "containerd"
)

var clog = log.WithComponent("NotificationClient")

// Client is used to notify a running agent process.
type Client interface {
	// Notify will send a notification that will be captured by the agent which will run a handler.
	Notify(ctx context.Context, message ipc.Message) error

	// GetID returns the identification for the notified agent.
	GetID() string
}

// NewAutoDetectedClient will try to detect the NRIA instance type and return a notification client for it.
func NewAutoDetectedClient(dockerAPIVersion, containerdNamespace, containerRuntime string) (Client, error) {
	pid, err := detection.GetInfraAgentProcess()
	if err != nil {
		return nil, err
	}

	clog.WithField("pid", pid).Info("found agent")

	inContainer, containerID, err := detection.IsContainerized(pid, dockerAPIVersion, config.DefaultDockerContainerdNamespace)
	if err != nil {
		clog.WithError(err).Info("Container ID not identified")
	}

	if inContainer {
		return NewContainerClient(dockerAPIVersion, containerdNamespace, containerID, containerRuntime)
	}
	return NewClient(int(pid))
}
