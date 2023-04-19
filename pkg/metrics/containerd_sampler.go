// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	metricTypes "github.com/newrelic/infrastructure-agent/pkg/metrics/types"
)

var (
	cslog = log.WithComponent("ContainerdSampler") //nolint:gochecknoglobals

	ErrContainerdSampler      = errors.New("containerd sampler error")
	ErrInitializeContainerd   = errors.New("Unable to initialize containerd client")
	ErrCannotGetPids          = errors.New("Unable to get pids for container")
	ErrCannotGetTask          = errors.New("Unable to get task for container")
	ErrCannotGetContainerInfo = errors.New("Unable to get container info")
)

type ContainerdSampler struct {
	containerdClientRetries int
	containerdClient        helpers.ContainerdInterface
	pidsCache               *pidsCache
	lastCacheClean          time.Time
}

func initializeContainerdClient() (helpers.ContainerdInterface, error) { //nolint:ireturn
	containerdClient := &helpers.ContainerdClient{}
	if err := containerdClient.Initialize(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContainerdSampler, err)
	}

	return containerdClient, nil
}

func NewContainerdSampler(cacheTTL time.Duration) ContainerSampler { //nolint:ireturn
	return NewContainerdSamplerWithClient(nil, cacheTTL)
}

func NewContainerdSamplerWithClient(client helpers.ContainerdInterface, cacheTTL time.Duration) ContainerSampler { //nolint:ireturn
	return &ContainerdSampler{ //nolint:exhaustruct
		containerdClient: client,
		lastCacheClean:   time.Now(),
		pidsCache:        newPidsCache(cacheTTL),
	}
}

// polls on containerd availability.
func (d *ContainerdSampler) Enabled() bool {
	if d.containerdClient != nil {
		return true
	}

	if d.containerdClientRetries > containerSamplerRetries {
		return false
	}
	d.containerdClientRetries++
	var err error

	d.containerdClient, err = initializeContainerdClient()
	if err != nil {
		cslog.WithError(err).Debug(ErrInitializeContainerd.Error())

		return false
	}

	return true
}

func (d *ContainerdSampler) NewDecorator() (ProcessDecorator, error) { //nolint:ireturn
	return newContainerdDecorator(d.containerdClient, d.pidsCache)
}

type containerdDecorator struct {
	containerdClient helpers.ContainerdInterface
	cache            *pidsCache
	pids             map[uint32]helpers.ContainerdMetadata
}

// compile-time assertion.
var _ ProcessDecorator = &containerdDecorator{} //nolint:exhaustruct

func newContainerdDecorator(containerdClient helpers.ContainerdInterface, pidsCache *pidsCache) (ProcessDecorator, error) { //nolint:ireturn
	dec := &containerdDecorator{ //nolint:exhaustruct
		containerdClient: containerdClient,
		cache:            pidsCache,
	}

	pids, err := dec.pidsContainers()
	if err != nil {
		return nil, err
	}
	dec.pids = pids

	return dec, nil
}

func (d *containerdDecorator) pidsContainers() (map[uint32]helpers.ContainerdMetadata, error) {
	pidsContainers := make(map[uint32]helpers.ContainerdMetadata)

	containersPerNamespace, err := d.containerdClient.Containers()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContainerdSampler, err)
	}

	for namespace, containers := range containersPerNamespace {
		for _, container := range containers {
			// For each container, get the PIDs
			err := d.pidsWithCache(container, namespace, pidsContainers)
			if err != nil {
				return nil, err
			}
		}

		// Remove cached data from old containers.
		d.cache.compact(len(containers))
	}

	return pidsContainers, nil
}

func (d *containerdDecorator) pidsWithCache(container containerd.Container, namespace string, pids map[uint32]helpers.ContainerdMetadata) error {
	// Get the PIDs for the container
	// If pids are in the cache (and not too old), reuse them
	if cached, ok := d.cache.get(container.ID()); ok {
		for _, pid := range cached {
			pids[pid] = helpers.ContainerdMetadata{Container: container, Namespace: namespace}
		}

		return nil
	}

	// Get the PIDs for the container
	ctx := namespaces.WithNamespace(context.Background(), namespace)
	task, err := container.Task(ctx, nil)
	if err != nil {
		cslog.WithError(err).WithField("container", container.ID()).Debug(ErrCannotGetTask.Error())

		return fmt.Errorf("%w: %v", ErrContainerdSampler, err)
	}

	pidsList, err := task.Pids(ctx)
	if err != nil {
		cslog.WithError(err).WithField("container", container.ID()).Debug(ErrCannotGetPids.Error())

		return fmt.Errorf("%w: %v", ErrContainerdSampler, err)
	}

	cachedPids := make([]uint32, 0, len(pidsList))

	for _, pid := range pidsList {
		pids[pid.Pid] = helpers.ContainerdMetadata{Container: container, Namespace: namespace}

		cachedPids = append(cachedPids, pid.Pid)
	}

	// Store fresh PIDs in the cache
	d.cache.put(container.ID(), cachedPids)

	return nil
}

func (d *containerdDecorator) Decorate(process *metricTypes.ProcessSample) {
	if containerMeta, ok := d.pids[uint32(process.ProcessID)]; ok {
		// Get container information
		cInfo, err := helpers.GetContainerdInfo(containerMeta)
		if err != nil {
			cslog.WithError(err).WithField("container", containerMeta.Container.ID()).Debug(ErrCannotGetContainerInfo.Error())
		}

		process.ContainerImage = cInfo.ImageID
		process.ContainerImageName = cInfo.ImageName
		process.ContainerLabels = cInfo.Labels
		process.ContainerID = cInfo.ID
		process.Contained = "true"
	}
}
