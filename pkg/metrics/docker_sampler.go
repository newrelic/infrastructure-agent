// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	metricTypes "github.com/newrelic/infrastructure-agent/pkg/metrics/types"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var (
	dslog = log.WithComponent("DockerSampler") //nolint:gochecknoglobals

	ErrNoPIDTitle    = errors.New("no PID title found")
	ErrDockerSampler = errors.New("docker sampler error")
)

type DockerSampler struct {
	dockerClientRetries int
	dockerClient        helpers.Docker
	pidsCache           *pidsCache
	lastCacheClean      time.Time
	apiVersion          string
}

func initializeDockerClient(apiVersion string) (helpers.Docker, error) { //nolint:ireturn
	dockerClient := &helpers.DockerClient{}
	if err := dockerClient.Initialize(apiVersion); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDockerSampler, err)
	}

	return dockerClient, nil
}

func NewDockerSampler(cacheTTL time.Duration, apiVersion string) ContainerSampler { //nolint:ireturn
	return NewDockerSamplerWithClient(nil, cacheTTL, apiVersion)
}

func NewDockerSamplerWithClient(client helpers.Docker, cacheTTL time.Duration, apiVersion string) ContainerSampler { //nolint:ireturn
	return &DockerSampler{ //nolint:exhaustruct
		dockerClient:   client,
		lastCacheClean: time.Now(),
		apiVersion:     apiVersion,
		pidsCache:      newPidsCache(cacheTTL),
	}
}

// polls on docker availability.
func (d *DockerSampler) Enabled() bool {
	if d.dockerClient != nil {
		return true
	}

	if d.dockerClientRetries > containerSamplerRetries {
		return false
	}
	d.dockerClientRetries++
	var err error

	d.dockerClient, err = initializeDockerClient(d.apiVersion)
	if err != nil {
		dslog.WithError(err).Info("unable to initialize docker client, maybe it is not installed on the host")

		return false
	}

	return true
}

func (d *DockerSampler) NewDecorator() (ProcessDecorator, error) { //nolint:ireturn
	return newDockerDecorator(d.dockerClient, d.pidsCache)
}

type dockerDecorator struct {
	dockerClient helpers.Docker
	cache        *pidsCache
	pids         map[uint32]types.Container
}

// compile-time assertion.
var _ ProcessDecorator = &dockerDecorator{} //nolint:exhaustruct

func newDockerDecorator(dockerClient helpers.Docker, cache *pidsCache) (ProcessDecorator, error) { //nolint:ireturn
	dec := &dockerDecorator{ //nolint:exhaustruct
		dockerClient: dockerClient,
		cache:        cache,
	}

	pids, err := dec.pidsContainers()
	if err != nil {
		return nil, err
	}
	dec.pids = pids

	return dec, nil
}

// topPids fills the pids map with the pids of the processes that run in the given container.
func (d *dockerDecorator) topPids(container types.Container, pids map[uint32]types.Container) error {
	// If pids are in cache (and not too old), we reuse them
	if cached, ok := d.cache.get(container.ID); ok {
		for _, pid := range cached {
			pids[pid] = container
		}

		return nil
	}

	titles, processes, err := d.dockerClient.ContainerTop(container.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDockerSampler, err)
	}
	pidColumn := -1

	for i, title := range titles {
		if title == "PID" {
			pidColumn = i

			break
		}
	}

	if pidColumn == -1 {
		return fmt.Errorf("%w for container %q top. Returned titles: %v", ErrNoPIDTitle, container.ID, titles)
	}
	cachedPids := make([]uint32, 0, len(processes))

	for _, process := range processes {
		pid, err := strconv.ParseUint(process[pidColumn], 10, 32)
		if err != nil {
			dslog.WithFieldsF(func() logrus.Fields {
				return logrus.Fields{
					"containerID": container.ID,
					"pid":         pid,
					"process":     process,
				}
			}).Debug("Wrong PID number. Ignoring.")

			continue
		}
		pidAsUint32 := uint32(pid)
		pids[pidAsUint32] = container

		cachedPids = append(cachedPids, pidAsUint32)
	}
	// store fresh pids in the cache
	d.cache.put(container.ID, cachedPids)

	return nil
}

// pidsContainers returns a map where each key is the PID of a process running in a container and the value is the
// container that contains it.
func (d *dockerDecorator) pidsContainers() (map[uint32]types.Container, error) {
	containers, err := d.dockerClient.Containers()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDockerSampler, err)
	}

	pids := map[uint32]types.Container{}
	for _, container := range containers {
		err := d.topPids(container, pids)
		if err != nil {
			return nil, err
		}
	}

	// remove cached data from old containers
	d.cache.compact(len(containers))

	return pids, nil
}

// Decorate adds container information to all the processes that belong to a container.
func (d *dockerDecorator) Decorate(process *metricTypes.ProcessSample) {
	if container, ok := d.pids[uint32(process.ProcessID)]; ok {
		imageIDComponents := strings.Split(container.ImageID, ":")
		process.ContainerImage = imageIDComponents[len(imageIDComponents)-1]
		process.ContainerImageName = container.Image
		process.ContainerLabels = container.Labels
		process.ContainerID = container.ID

		if len(container.Names) > 0 {
			process.ContainerName = strings.TrimPrefix(container.Names[0], "/")
		}
		process.Contained = "true"
	}
}
