// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"context"
	"os"
	"runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
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
	Containers() ([]types.Container, error)
	ContainerTop(containerID string) (titles []string, processes [][]string, err error)
}

type DockerClient struct {
	client *client.Client
}

func (dc *DockerClient) Initialize(apiVersion string) (err error) {
	if !IsDockerRunning() {
		return ErrNoDockerd
	}

	dc.client, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion(apiVersion))
	if err != nil {
		return errors.Wrap(err, "failed to initialize docker client")
	}

	return
}

func (dc *DockerClient) Containers() ([]types.Container, error) {
	return dc.client.ContainerList(context.Background(), types.ContainerListOptions{})
}

func (dc *DockerClient) ContainerTop(containerID string) (titles []string, processes [][]string, err error) {
	body, err := dc.client.ContainerTop(context.Background(), containerID, []string{})
	if err != nil {
		return nil, nil, err
	}
	return body.Titles, body.Processes, nil
}

func IsDockerRunning() bool {
	if runtime.GOOS == "windows" {
		_, err := os.Stat(windowsDockerSocket)
		return err == nil
	} else {
		_, err := os.Stat(linuxDockerSocket)
		return err == nil
	}
}
