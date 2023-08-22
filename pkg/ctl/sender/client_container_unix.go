// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package sender

func NewContainerClient(dockerAPIVersion, containerdNamespace, containerID, runtime string) (Client, error) {
	if runtime == RuntimeContainerd {
		return NewContainerdClient(containerdNamespace, containerID)
	}
	return NewDockerClient(dockerAPIVersion, containerID)
}
