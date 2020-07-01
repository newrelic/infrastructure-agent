// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fargate

import "time"

//TaskMetadata https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v2.html#task-metadata-endpoint-v2-response
type TaskMetadata struct {

	// The name of the cluster that hosts the task.
	Cluster string `json:"Cluster"`

	// The Amazon Resource Name (ARN) of the task.
	TaskArn string `locationName:"taskArn" type:"string"`

	// The family of the Amazon ECS task definition for the task.
	Family string `locationName:"family" type:"string"`

	// The revision of the Amazon ECS task definition for the task.
	Revision string `locationName:"revision" type:"string"`

	// The desired status of the task. For more information, see Task Lifecycle
	// (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_life_cycle.html).
	DesiredStatus string `locationName:"desiredStatus" type:"string"`

	// The known status for the task from Amazon ECS.
	KnownStatus string `locationName:"knownStatus" type:"string"`

	// A list of container metadata for each container associated with the task.
	Containers []awsContainer `json:"Containers"`

	// The resource limits specified at the task level (such as CPU and memory). This parameter is omitted if no resource limits are defined.
	Limits map[string]float64 `json:"Limits"`

	// The Unix timestamp for when the container image pull began.
	PullStartedAt *time.Time `locationName:"pullStartedAt" type:"timestamp"`

	// The Unix timestamp for when the container image pull completed.
	PullStoppedAt *time.Time `locationName:"pullStoppedAt" type:"timestamp"`

	// The Unix timestamp for when the task execution stopped.
	ExecutionStoppedAt *time.Time `locationName:"executionStoppedAt" type:"timestamp"`
}

// Container as defined by the ECS metadata API
type awsContainer struct {

	// The Docker ID for the container.
	DockerID string `json:"DockerID"`

	// The name of the container as specified in the task definition.
	Name string `json:"Name"`

	// The name of the container supplied to Docker.
	// The Amazon ECS container agent generates a unique name for the container to avoid name collisions when multiple copies of the same task definition are run on a single instance.
	DockerName string `json:"DockerName"`

	// The image for the container.
	Image string `json:"Image"`

	//The SHA-256 digest for the image.
	ImageID string `json:"ImageID,omitempty"`

	// Any ports exposed for the container. This parameter is omitted if there are no exposed ports.
	Ports []PortResponse `json:"Ports"`

	// Any labels applied to the container. This parameter is omitted if there are no labels applied.
	Labels map[string]string `json:"Labels"`

	// Any labels applied to the container. This parameter is omitted if there are no labels applied.
	Limits map[string]uint64 `json:"Limits"`

	// The desired status for the container from Amazon ECS.
	DesiredStatus string `json:"DesiredStatus"`

	// The known status for the container from Amazon ECS.
	KnownStatus string `json:"KnownStatus"`

	// The exit code for the container. This parameter is omitted if the container has not exited.
	ExitCode string `json:"ExitCode"`

	// The time stamp for when the container was created. This parameter is omitted if the container has not been created yet.
	CreatedAt string `json:"CreatedAt"`

	// The time stamp for when the container started. This parameter is omitted if the container has not started yet.
	StartedAt string `json:"StartedAt"` // 2017-11-17T17:14:07.781711848Z

	// The time stamp for when the container stopped. This parameter is omitted if the container has not stopped yet.
	FinishedAt string `json:"FinishedAt"`

	// The type of the container. Containers that are specified in your task definition are of type NORMAL.
	// You can ignore other container types, which are used for internal task resource provisioning by the Amazon ECS container agent.
	Type string `json:"Type"`

	// The network information for the container, such as the network mode and IP address. This parameter is omitted if no network information is defined.
	Networks []Network `json:"Networks"`
}

// Network information of the container
type Network struct {

	// NetworkMode currently only supported mode is awsvpc
	NetworkMode string `json:"NetworkMode"`

	// IPv4 Addresses supplied in a single element list
	IPv4Addresses []string `json:"IPv4Addresses"`
}

// PortResponse defines the schema for portmapping response JSON
// object
type PortResponse struct {
	ContainerPort uint16
	Protocol      string
	HostPort      uint16 `json:",omitempty"`
}
