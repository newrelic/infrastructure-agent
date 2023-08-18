// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package sender

const (
	Runtime_docker     = "docker"
	Runtime_containerd = "containerd"
)

func NewContainerClient(dockerAPIVersion, containerdNamespace, containerID, runtime string) (Client, error) {
	if runtime == Runtime_containerd {
		return NewContainerdClient(containerdNamespace, containerID)
	}
	return NewDockerClient(dockerAPIVersion, containerID)
}
