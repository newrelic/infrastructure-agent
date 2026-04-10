// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"context"
	"os"
	"runtime"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/pkg/errors"
)

const (
	linuxDockerSocket   = "/var/run/docker.sock"
	windowsDockerSocket = "//./pipe/docker_engine"
)

var (
	ErrNoDockerd = errors.New("no active docker instance found")
)

type Docker interface {
	Initialize(apiVersion string) error
	Containers() ([]container.Summary, error)
	ContainerTop(containerID string) (titles []string, processes [][]string, err error)
}

type DockerClient struct {
	client *client.Client
}

func (dc *DockerClient) Initialize(apiVersion string) (err error) {
	if !IsDockerRunning() {
		return ErrNoDockerd
	}

	dc.client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return errors.Wrap(err, "failed to initialize docker client")
	}

	return
}

func (dc *DockerClient) Containers() ([]container.Summary, error) {
	//nolint:exhaustruct // zero-value options is the intended default
	result, err := dc.client.ContainerList(context.Background(), client.ContainerListOptions{})
	if err != nil {
		return nil, err //nolint:wrapcheck // passthrough from docker client
	}

	return result.Items, nil
}

func (dc *DockerClient) ContainerTop(containerID string) (titles []string, processes [][]string, err error) {
	//nolint:exhaustruct // zero-value options is the intended default
	body, err := dc.client.ContainerTop(
		context.Background(), containerID, client.ContainerTopOptions{},
	)
	if err != nil {
		return nil, nil, err
	}

	return body.Titles, body.Processes, nil
}

func IsDockerRunning() bool {
	if runtime.GOOS == "windows" {
		_, err := os.Stat(windowsDockerSocket)
		return err == nil
	}

	dockerSock, err := os.Stat(linuxDockerSocket)
	if err != nil {
		return false
	}

	if dockerSock.Mode()&os.ModeSocket == 0 {
		return false
	}

	return true
}
