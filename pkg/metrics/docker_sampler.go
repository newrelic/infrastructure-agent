// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	metricTypes "github.com/newrelic/infrastructure-agent/pkg/metrics/types"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/lru"
	"github.com/sirupsen/logrus"
)

var dslog = log.WithComponent("DockerSampler")

type ContainerSampler interface {
	Enabled() bool
	NewDecorator() (ProcessDecorator, error)
}

type DockerSampler struct {
	dockerClientRetries int
	dockerClient        helpers.Docker
	pidsCache           *pidsCache
	lastCacheClean      time.Time
	apiVersion          string
}

func initializeDockerClient(apiVersion string) (dockerClient helpers.Docker, err error) {
	dockerClient = &helpers.DockerClient{}
	if err = dockerClient.Initialize(apiVersion); err != nil {
		return nil, err
	}
	return dockerClient, nil
}

func NewDockerSampler(cacheTTL time.Duration, apiVersion string) ContainerSampler {
	return NewDockerSamplerWithClient(nil, cacheTTL, apiVersion)
}

func NewDockerSamplerWithClient(client helpers.Docker, cacheTTL time.Duration, apiVersion string) ContainerSampler {
	return &DockerSampler{
		dockerClient:   client,
		lastCacheClean: time.Now(),
		apiVersion:     apiVersion,
		pidsCache:      newPidsCache(cacheTTL),
	}
}

// polls on docker availability
func (d *DockerSampler) Enabled() bool {
	if d.dockerClient != nil {
		return true
	}

	if d.dockerClientRetries > 100 {
		return false
	}
	d.dockerClientRetries++

	var err error
	d.dockerClient, err = initializeDockerClient(d.apiVersion)
	if err != nil {
		dslog.WithError(err).Debug("Unable to initialize docker client.")
		return false
	}
	return true
}

func (d *DockerSampler) NewDecorator() (ProcessDecorator, error) {
	return newDecoratorImpl(d.dockerClient, d.pidsCache)
}

type ProcessDecorator interface {
	Decorate(process *metricTypes.ProcessSample)
}

type decoratorImpl struct {
	dockerClient helpers.Docker
	cache        *pidsCache
	pids         map[int32]types.Container
}

var _ ProcessDecorator = &decoratorImpl{} // compile-time assertion

func newDecoratorImpl(dockerClient helpers.Docker, cache *pidsCache) (ProcessDecorator, error) {
	d := &decoratorImpl{
		dockerClient: dockerClient,
		cache:        cache,
	}
	pids, err := d.pidsContainers()
	if err != nil {
		return nil, err
	}
	d.pids = pids
	return d, nil
}

// topPids fills the pids map with the pids of the processes that run in the given container
func (d *decoratorImpl) topPids(container types.Container, pids map[int32]types.Container) error {
	// If pids are in cache (and not too old), we reuse them
	if cached, ok := d.cache.get(container.ID); ok {
		for _, pid := range cached {
			pids[pid] = container
		}
		return nil
	}

	titles, processes, err := d.dockerClient.ContainerTop(container.ID)
	if err != nil {
		return err
	}
	pidColumn := -1
	for i, title := range titles {
		if title == "PID" {
			pidColumn = i
			break
		}
	}
	if pidColumn == -1 {
		return fmt.Errorf("no PID title found for container %q top. Returned titles: %v", container.ID, titles)
	}
	cachedPids := make([]int32, 0, len(processes))
	for _, process := range processes {
		pid, err := strconv.ParseInt(process[pidColumn], 10, 32)
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
		pidAsInt32 := int32(pid)
		pids[pidAsInt32] = container
		cachedPids = append(cachedPids, pidAsInt32)
	}
	// store fresh pids in the cache
	d.cache.put(container.ID, cachedPids)
	return nil
}

// pidsContainers returns a map where each key is the PID of a process running in a container and the value is the
// container that contains it
func (d *decoratorImpl) pidsContainers() (map[int32]types.Container, error) {
	containers, err := d.dockerClient.Containers()
	if err != nil {
		return nil, err
	}

	pids := map[int32]types.Container{}
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

// DecorateProcesses adds container information to all the processes that belong to a container
func (d *decoratorImpl) Decorate(process *metricTypes.ProcessSample) {
	if container, ok := d.pids[process.ProcessID]; ok {
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

// Caching container PID samples with an LRU cache with an associated TTL
type pidsCache struct {
	ttl   time.Duration
	cache *lru.Cache
}

func newPidsCache(ttl time.Duration) *pidsCache {
	return &pidsCache{ttl: ttl, cache: lru.New()}
}

type pidsCacheEntry struct {
	creationTime time.Time
	pids         []int32
}

func (p *pidsCache) get(containerID string) ([]int32, bool) {
	value, ok := p.cache.Get(containerID)
	if !ok || p.ttl == 0 {
		return nil, false
	}

	// Early random cache expiration to minimize Cache Stampede Risk (cache entries may expire 33% before)
	rndTTL := 2*p.ttl/3 - time.Duration(rand.Int63n(int64(p.ttl/3)))

	entry := value.(*pidsCacheEntry)
	if time.Now().After(entry.creationTime.Add(rndTTL)) {
		return nil, false
	}
	return entry.pids, true
}

func (p *pidsCache) put(containerId string, pids []int32) {
	entry := &pidsCacheEntry{
		creationTime: time.Now(),
		pids:         pids,
	}
	p.cache.Add(containerId, entry)
}

func (p *pidsCache) compact(size int) {
	p.cache.RemoveUntilLen(size)
}
