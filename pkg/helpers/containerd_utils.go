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
	UnixContainerdSocket = "/run/containerd/containerd.sock"
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

// ContainerdInterface is the interface for containerd operations.
type ContainerdInterface interface {
	Initialize() error
	Containers(...namespace) (map[namespace][]containerd.Container, error)
	Namespaces() ([]namespace, error)
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

	client, err := containerd.New(UnixContainerdSocket)
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

		nss = allNamespaces
	}

	return cc.containersFromNamespaces(nss)
}

func (cc *ContainerdClient) containersFromNamespaces(nss []string) (map[namespace][]containerd.Container, error) {
	containersPerNamespace := map[namespace][]containerd.Container{}

	for _, ns := range nss {
		ctx := namespaces.WithNamespace(context.Background(), ns)

		containers, err := cc.client.Containers(ctx)
		if err != nil {
			return nil, containerdError(err)
		}

		containersPerNamespace[ns] = containers
	}

	return containersPerNamespace, nil
}

func IsContainerdRunning() bool {
	if runtime.GOOS == "windows" {
		return false
	}

	sock, err := os.Stat(UnixContainerdSocket)
	if err != nil {
		return false
	}

	return (sock.Mode() & os.ModeSocket) != 0
}

// GetContainerdInfo returns detailed information about a container.
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
