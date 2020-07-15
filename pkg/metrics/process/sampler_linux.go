// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package process

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	metrics "github.com/newrelic/infrastructure-agent/pkg/metrics"
	sampler "github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

// processSampler is an implementation of the metrics_sender.Sampler interface, which returns runtime information about
// the currently running processes
type processSampler struct {
	harvest          Harvester
	containerSampler metrics.ContainerSampler
	lastRun          time.Time
	hasAlreadyRun    bool
	interval         time.Duration
	cache            *cache
}

var (
	_                       sampler.Sampler = (*processSampler)(nil) // static interface assertion
	containerNotRunningErrs                 = map[string]struct{}{}
)

// NewProcessSampler creates and returns a new process Sampler, given an agent context.
func NewProcessSampler(ctx agent.AgentContext) sampler.Sampler {
	hasConfig := ctx != nil && ctx.Config() != nil

	ttlSecs := config.DefaultContainerCacheMetadataLimit
	apiVersion := ""
	interval := config.FREQ_INTERVAL_FLOOR_PROCESS_METRICS
	if hasConfig {
		cfg := ctx.Config()
		ttlSecs = cfg.ContainerMetadataCacheLimit
		apiVersion = cfg.DockerApiVersion
		interval = cfg.MetricsProcessSampleRate
	}
	cache := newCache()
	harvest := newHarvester(ctx, &cache)
	dockerSampler := metrics.NewDockerSampler(time.Duration(ttlSecs)*time.Second, apiVersion)

	return &processSampler{
		harvest:          harvest,
		containerSampler: dockerSampler,
		cache:            &cache,
		interval:         time.Second * time.Duration(interval),
	}

}

func (ps *processSampler) OnStartup() {}

func (ps *processSampler) Name() string {
	return "ProcessSampler"
}

func (ps *processSampler) Interval() time.Duration {
	return ps.interval
}

func (ps *processSampler) Disabled() bool {
	return ps.Interval() <= config.FREQ_DISABLE_SAMPLING
}

// Sample returns samples for all the running processes, decorated with Docker runtime information, if applies.
func (ps *processSampler) Sample() (results sample.EventBatch, err error) {
	var elapsedMs int64
	var elapsedSeconds float64
	now := time.Now()
	if ps.hasAlreadyRun {
		elapsedMs = (now.UnixNano() - ps.lastRun.UnixNano()) / 1000000
	}
	elapsedSeconds = float64(elapsedMs) / 1000
	ps.lastRun = now

	pids, err := ps.harvest.Pids()
	if err != nil {
		return nil, err
	}

	var dockerDecorator metrics.ProcessDecorator = nil
	if ps.containerSampler.Enabled() {
		dockerDecorator, err = ps.containerSampler.NewDecorator()
		if err != nil {
			if id := containerIDFromNotRunningErr(err); id != "" {
				if _, ok := containerNotRunningErrs[id]; !ok {
					containerNotRunningErrs[id] = struct{}{}
					mplog.WithError(err).Warn("instantiating docker sampler process decorator")
				}
			} else {
				mplog.WithError(err).Warn("instantiating docker sampler process decorator")
				if strings.Contains(err.Error(), "client is newer than server") {
					mplog.WithError(err).Error("Only docker api version from 1.24 upwards are officially supported. You can still use the docker_api_version configuration to work with older versions. You can check https://docs.docker.com/develop/sdk/ what api version maps with each docker version.")
				}
			}
		}
	}

	for _, pid := range pids {
		var sample *types.ProcessSample
		var err error

		sample, err = ps.harvest.Do(pid, elapsedSeconds)
		if err != nil {
			mplog.WithError(err).WithField("pid", pid).Debug("Skipping process.")
			continue
		}

		if dockerDecorator != nil {
			dockerDecorator.Decorate(sample)
		}

		results = append(results, ps.normalizeSample(sample))
	}

	ps.cache.items.RemoveUntilLen(len(pids))
	ps.hasAlreadyRun = true
	return results, nil
}

func (self *processSampler) normalizeSample(s *types.ProcessSample) sample.Event {
	if len(s.ContainerLabels) > 0 {
		sb, err := json.Marshal(s)
		if err == nil {
			bm := &FlatProcessSample{}
			if err = json.Unmarshal(sb, bm); err == nil {
				for name, value := range s.ContainerLabels {
					key := fmt.Sprintf("containerLabel_%s", name)
					(*bm)[key] = value
				}
				return bm
			}
		} else {
			mplog.WithError(err).WithField("sample", fmt.Sprintf("%+v", s)).Debug("normalizeSample can't operate on the sample.")
		}
	}
	return s
}

// FlatProcessSample stores the process sampling information as a map
type FlatProcessSample map[string]interface{}

var _ sample.Event = &FlatProcessSample{} // FlatProcessSample implements sample.Event

func (f *FlatProcessSample) Type(eventType string) {
	(*f)["eventType"] = eventType
}

func (f *FlatProcessSample) Entity(key entity.Key) {
	(*f)["entityKey"] = key
}

func (f *FlatProcessSample) Timestamp(timestamp int64) {
	(*f)["timestamp"] = timestamp
}

func containerIDFromNotRunningErr(err error) string {
	prefix := "Error response from daemon: Container "
	suffix := " is not running"
	msg := err.Error()
	i := strings.Index(msg, prefix)
	j := strings.Index(msg, suffix)
	if i == -1 || j == -1 {
		return ""
	}
	return msg[len(prefix):j]
}
