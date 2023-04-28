// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
)

// ContainerdSocket is the default socket for containerd.
const (
	unixContainerdSocket  = "/run/containerd/containerd.sock"
	windowsContainerdPipe = `\\.\pipe\containerd-containerd`
)

// ErrNoContainerd is returned when containerd is not running.
var ErrNoContainerd = errors.New("no active containerd instance found")

type ContainerdMetadata struct {
	Container containerd.Container
	Namespace string
}

type ContainerInfo struct {
	ID        string
	Namespace string
	ImageName string
	ImageID   string
	Labels    map[string]string
}

type namespace = string

// Containerd is the interface for containerd operations.
type ContainerdInterface interface {
	Initialize() error
	Namespaces() ([]string, error)
	Containers(...string) (map[namespace][]containerd.Container, error)
	ContainerProcesses(containerID string) ([]containerd.ProcessInfo, error)
}

// ContainerdClient is the client for containerd.
type ContainerdClient struct {
	client *containerd.Client
}

// containerdError wraps the error returned by containerd.
func containerdError(err error) error {
	return fmt.Errorf("containerd error: %w", err)
}

// Initialize initializes the containerd client.
func (cc *ContainerdClient) Initialize() error {
	if !IsContainerdRunning() {
		return containerdError(ErrNoContainerd)
	}

	client, err := containerd.New(unixContainerdSocket)
	if err != nil {
		return containerdError(err)
	}

	cc.client = client

	return nil
}

// Namespaces returns the list of namespaces.
func (cc *ContainerdClient) Namespaces() ([]string, error) {
	namespaces, err := cc.client.NamespaceService().List(context.Background())
	if err != nil {
		return nil, containerdError(err)
	}

	return namespaces, nil
}

// Containers returns the list of containers per namespace. A list of namespaces can be provided
// as arguments. If no namespace is provided, all containers are returned.
func (cc *ContainerdClient) Containers(nss ...string) (map[namespace][]containerd.Container, error) {
	if len(nss) == 0 {

		allNamespaces, err := cc.Namespaces()
		if err != nil {
			return nil, containerdError(err)
		}

		return cc.containersFromNamespaces(allNamespaces)
	}

	return cc.containersFromNamespaces(nss)
}

func (cc *ContainerdClient) containersFromNamespaces(nss []string) (map[namespace][]containerd.Container, error) {
	containersPerNamespace := map[namespace][]containerd.Container{}

	for _, namespace := range nss {
		ctx := namespaces.WithNamespace(context.Background(), namespace)

		containers, err := cc.client.Containers(ctx)
		if err != nil {
			return nil, containerdError(err)
		}

		containersPerNamespace[namespace] = containers
	}

	return containersPerNamespace, nil
}

func IsContainerdRunning() bool {
	if runtime.GOOS == "windows" {
		_, err := os.Stat(windowsContainerdPipe)

		return err == nil
	}

	sock, err := os.Stat(unixContainerdSocket)
	if err != nil {
		return false
	}

	return sock.Mode()&os.ModeSocket != 0
}

// getContainerInfo returns detailed information about a container.
func GetContainerdInfo(containerMeta ContainerdMetadata) (ContainerInfo, error) {
	containerID := containerMeta.Container.ID()

	ctx := namespaces.WithNamespace(context.Background(), containerMeta.Namespace)

	containerLabels, err := containerMeta.Container.Labels(ctx)
	if err != nil {
		return ContainerInfo{}, containerdError(err)
	}

	containerImage, err := containerMeta.Container.Image(ctx)
	if err != nil {
		return ContainerInfo{}, containerdError(err)
	}

	containerImageName := containerImage.Name()
	containerImageID := containerImage.Target().Digest.String()

	return ContainerInfo{
		ID:        containerID,
		Namespace: containerMeta.Namespace,
		ImageName: containerImageName,
		ImageID:   containerImageID,
		Labels:    containerLabels,
	}, nil
}

// ContainerProcesses returns the processes for a container.
func (cc *ContainerdClient) ContainerProcesses(containerID string) ([]containerd.ProcessInfo, error) {
	container, err := cc.client.LoadContainer(context.Background(), containerID)
	if err != nil {
		return nil, containerdError(err)
	}

	task, err := container.Task(context.Background(), nil)
	if err != nil {
		return nil, containerdError(err)
	}

	processes, err := task.Pids(context.Background())
	if err != nil {
		return nil, containerdError(err)
	}

	return processes, nil
}
