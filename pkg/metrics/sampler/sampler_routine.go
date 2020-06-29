// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sampler

import (
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

type SamplerRoutine struct {
	name           string
	stopChannel    chan bool
	waitForCleanup *sync.WaitGroup
}

var mslog = log.WithField("component", "Sampler routine")

func StartSamplerRoutine(sampler Sampler, sampleQueue chan sample.EventBatch) *SamplerRoutine {
	sr := &SamplerRoutine{
		name:           sampler.Name(),
		stopChannel:    make(chan bool),
		waitForCleanup: &sync.WaitGroup{},
	}

	sampler.OnStartup()

	sr.waitForCleanup.Add(1)

	go func() {
		ticker := time.NewTicker(sampler.Interval())
		defer func() {
			ticker.Stop()
			sr.waitForCleanup.Done()
		}()
		mslog.WithField("name", sr.name).Debug("Started sampler routine.")
		for {
			select {
			case <-ticker.C:
				samples, err := sampler.Sample()
				if err != nil {
					mslog.WithError(err).WithField("samplerName", sr.name).Error("can't get sample from sampler")
					continue
				}
				select {
				case sampleQueue <- samples:
				case <-sr.stopChannel:
					return
				}
			case <-sr.stopChannel:
				return
			}
		}
	}()

	return sr
}

func (sr *SamplerRoutine) Stop() {
	close(sr.stopChannel)
	sr.waitForCleanup.Wait()
	sr.stopChannel = nil
	mslog.WithField("name", sr.name).Debug("Stopped sampler routine.")
}
