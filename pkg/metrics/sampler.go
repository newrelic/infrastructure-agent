// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"math/rand"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/helpers/lru"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	metricTypes "github.com/newrelic/infrastructure-agent/pkg/metrics/types"
)

const containerSamplerRetries = 100

type ContainerSampler interface {
	Enabled() bool
	NewDecorator() (ProcessDecorator, error)
}

// GetContainerSampler returns the first available container sampler.
func GetContainerSampler(cacheTTL time.Duration, dockerAPIVersion string) ContainerSampler { //nolint:ireturn
	clog := log.WithComponent("ContainerSampler")

	containerdSampler := NewContainerdSampler(cacheTTL)
	if containerdSampler.Enabled() {
		return containerdSampler
	}

	// containerd seems to not be enabled. Trying with docker API
	clog.Debug("containerd seems to not be present. Trying docker-based container sampler")

	dockerSampler := NewDockerSampler(cacheTTL, dockerAPIVersion)
	if dockerSampler.Enabled() {
		return dockerSampler
	}

	// No more container runtimes available, returning default docker sampler
	clog.Debug("No container runtimes available, returning default, containerd-based container sampler")

	return containerdSampler
}

type ProcessDecorator interface {
	Decorate(process *metricTypes.ProcessSample)
}

// Caching container PID samples with an LRU cache with an associated TTL.
type pidsCache struct {
	ttl   time.Duration
	cache *lru.Cache
}

func newPidsCache(ttl time.Duration) *pidsCache {
	return &pidsCache{ttl: ttl, cache: lru.New()}
}

type pidsCacheEntry struct {
	creationTime time.Time
	pids         []uint32
}

func (p *pidsCache) get(containerID string) ([]uint32, bool) {
	value, ok := p.cache.Get(containerID)
	if !ok || p.ttl == 0 {
		return nil, false
	}

	// Early random cache expiration to minimize Cache Stampede Risk (cache entries may expire 33% before)
	rndTTL := 2*p.ttl/3 - time.Duration(rand.Int63n(int64(p.ttl/3))) //nolint:gosec,gomnd

	entry, isEntry := value.(*pidsCacheEntry)
	if !isEntry {
		return nil, false
	}

	if time.Now().After(entry.creationTime.Add(rndTTL)) {
		return nil, false
	}

	return entry.pids, true
}

func (p *pidsCache) put(containerID string, pids []uint32) {
	entry := &pidsCacheEntry{
		creationTime: time.Now(),
		pids:         pids,
	}
	p.cache.Add(containerID, entry)
}

func (p *pidsCache) compact(size int) {
	p.cache.RemoveUntilLen(size)
}
