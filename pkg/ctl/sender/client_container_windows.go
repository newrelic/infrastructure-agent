// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sender

func NewContainerClient(dockerAPIVersion, containerdNamespace, containerID, runtime string) (Client, error) {
	return NewDockerClient(dockerAPIVersion, containerID)
}
