// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package process

import (
	"errors"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

// processSampler is an implementation of the metrics_sender.Sampler interface, which returns runtime information about
// the currently running processes
type processSampler struct {
	harvest       Harvester
	lastRun       time.Time
	hasAlreadyRun bool
	interval      time.Duration
}

var (
	_ sampler.Sampler = (*processSampler)(nil) // static interface assertion
)

// NewProcessSampler creates and returns a new process Sampler, given an agent context.
func NewProcessSampler(ctx agent.AgentContext) sampler.Sampler {
	hasConfig := ctx != nil && ctx.Config() != nil

	apiVersion := ""
	interval := config.FREQ_INTERVAL_FLOOR_PROCESS_METRICS
	if hasConfig {
		cfg := ctx.Config()
		apiVersion = cfg.DockerApiVersion
		interval = cfg.MetricsProcessSampleRate
	}
	harvest := newHarvester(ctx)

	return &processSampler{
		harvest:  harvest,
		interval: time.Second * time.Duration(interval),
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

	for _, pid := range pids {
		var processSample *types.ProcessSample
		var err error

		processSample, err = ps.harvest.Do(pid, elapsedSeconds)
		if err != nil {
			procLog := mplog.WithError(err)
			if errors.Is(err, errProcessWithoutRSS) {
				procLog = procLog.WithField(config.TracesFieldName, config.ProcessTrace)
			}

			procLog.WithField("pid", pid).Debug("Skipping process.")
			continue
		}

		results = append(results, ps.normalizeSample(processSample))
	}

	ps.hasAlreadyRun = true
	return results, nil
}
