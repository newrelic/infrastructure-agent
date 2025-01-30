// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	metricTypes "github.com/newrelic/infrastructure-agent/pkg/metrics/types"
)

var (
	cslog = log.WithComponent("ContainerdSampler") //nolint:gochecknoglobals

	errContainerdSampler      = errors.New("containerd sampler")
	errInitializeContainerd   = errors.New("unable to initialize containerd client, maybe it is not installed on the host")
	errCannotGetPids          = errors.New("unable to get pids for container")
	errCannotGetTask          = errors.New("unable to get task for container")
	errCannotGetContainerInfo = errors.New("unable to get container info")
)

type ContainerdSampler struct {
	containerdClientRetries int
	containerdClient        *helpers.ContainerdClient
	pidsCache               *pidsCache
	lastCacheClean          time.Time
	dockerNamespace         string
}

func initializeContainerdClient() (*helpers.ContainerdClient, error) {
	containerdClient := &helpers.ContainerdClient{}
	if err := containerdClient.Initialize(); err != nil {
		return nil, fmt.Errorf("%s: %w", errContainerdSampler.Error(), err)
	}

	return containerdClient, nil
}

func NewContainerdSampler(cacheTTL time.Duration, dockerNamespace string) *ContainerdSampler {
	return NewContainerdSamplerWithClient(nil, cacheTTL, dockerNamespace)
}

func NewContainerdSamplerWithClient(client *helpers.ContainerdClient, cacheTTL time.Duration, dockerNamespace string) *ContainerdSampler {
	return &ContainerdSampler{ //nolint:exhaustruct
		containerdClient: client,
		lastCacheClean:   time.Now(),
		pidsCache:        newPidsCache(cacheTTL),
		dockerNamespace:  dockerNamespace,
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
		cslog.WithError(err).Info(errInitializeContainerd.Error())

		return false
	}

	return true
}

func (d *ContainerdSampler) NewDecorator() (ProcessDecorator, error) { //nolint:ireturn
	return newContainerdDecorator(d.containerdClient, d.pidsCache, d.dockerNamespace)
}

type containerdDecorator struct {
	containerdClient helpers.ContainerdInterface
	cache            *pidsCache
	pids             map[uint32]helpers.ContainerdMetadata
	dockerNamespace  string
}

// compile-time assertion.
var _ ProcessDecorator = &containerdDecorator{} //nolint:exhaustruct

func newContainerdDecorator(containerdClient helpers.ContainerdInterface, pidsCache *pidsCache, dockerNamespace string) (ProcessDecorator, error) { //nolint:ireturn
	dec := &containerdDecorator{ //nolint:exhaustruct
		containerdClient: containerdClient,
		cache:            pidsCache,
		dockerNamespace:  dockerNamespace,
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
		return nil, fmt.Errorf("%s: %w", errContainerdSampler.Error(), err)
	}

	for namespace, containers := range containersPerNamespace {
		if namespace == d.dockerNamespace {
			continue
		}
		for _, container := range containers {
			// For each container, get the PIDs
			err := d.pidsWithCache(container, namespace, pidsContainers)
			if err != nil {
				// If no task is found for a given container, there is no execution instance of it.
				if errdefs.IsNotFound(err) {
					continue
				}
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
		cslog.WithError(err).WithField("container", container.ID()).Debug(errCannotGetTask.Error())

		return fmt.Errorf("%s: %w", errContainerdSampler.Error(), err)
	}

	pidsList, err := task.Pids(ctx)
	if err != nil {
		cslog.WithError(err).WithField("container", container.ID()).Debug(errCannotGetPids.Error())

		return fmt.Errorf("%s: %w", errContainerdSampler.Error(), err)
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

// Decorate adds container information to all the processes that belong to a container.
func (d *containerdDecorator) Decorate(process *metricTypes.ProcessSample) {
	if containerMeta, ok := d.pids[uint32(process.ProcessID)]; ok {
		// Get container information
		cInfo, err := helpers.GetContainerdInfo(containerMeta)
		if err != nil {
			cslog.WithError(err).WithField("container", containerMeta.Container.ID()).Debug(errCannotGetContainerInfo.Error())
		}

		process.ContainerImage = cInfo.ImageID
		process.ContainerImageName = cInfo.ImageName
		process.ContainerLabels = cInfo.Labels
		process.ContainerID = cInfo.ID
		// seems that containerd does not distinguish container name and container ID
		process.ContainerName = cInfo.ID
		process.Contained = "true"
	}
}
