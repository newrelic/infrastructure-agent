// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics_sender

import (
	"fmt"

	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

const (
	SAMPLE_QUEUE_CAPACITY = 10 // Number of sample batches we'll wait for, min 2 * high freq samplers + 1 * low freq samples
)

var slog = log.WithField("component", "Metrics Sender")

// Sender is responsible for submitting data to the collector endpoint.
type Sender struct {
	ctx                  agent.AgentContext
	internalRoutineWaits *sync.WaitGroup // Waitgroup to keep track of how many goroutines are running and wait for them to stop
	stopChannel          chan bool       // Channel will be closed when we want to stop all internal goroutines
	sampleQueue          chan sample.EventBatch
	samplers             []sampler.Sampler
}

func NewSender(ctx agent.AgentContext) *Sender {
	return &Sender{
		ctx:                  ctx,
		sampleQueue:          make(chan sample.EventBatch, SAMPLE_QUEUE_CAPACITY),
		internalRoutineWaits: &sync.WaitGroup{},
	}
}

func (s *Sender) RegisterSampler(sampler sampler.Sampler) {
	// don't even register the sampler if it's disabled
	if sampler.Disabled() {
		slog.WithField("sampler", sampler.Name()).Warn("Sampler is disabled and will not run")
		return
	}

	s.samplers = append(s.samplers, sampler)
}

// Start will register the sender with the collector, then start a couple of background
// routines to handle incoming data and post it to the server periodically.
func (s *Sender) Start() (err error) {
	if s.stopChannel != nil {
		return fmt.Errorf("Cannot start sender: The sender is already running. (stopChannel is not nil)")
	}

	// Set up the stop channel so the routines can wait for it to be closed
	s.stopChannel = make(chan bool)

	go func() {
		s.internalRoutineWaits.Add(1)
		s.scheduleSamplers()
		s.internalRoutineWaits.Done()
	}()

	return
}

// Stop will gracefully shut down all sending processes and reset the state of the sender.
// After Stop() returns, it is safe to call Start() again on the same sender instance.
func (s *Sender) Stop() (err error) {
	if s.stopChannel == nil {
		return fmt.Errorf("Cannot stop sender: The sender is not running. (stopChannel is nil)")
	}

	close(s.stopChannel)
	s.internalRoutineWaits.Wait()
	s.stopChannel = nil

	return
}

// Periodically gather all samples and send them to Insights
func (s *Sender) scheduleSamplers() {
	var samplerRoutines []*sampler.SamplerRoutine

	for _, t := range s.samplers {
		slog.WithField("sampler", t.Name()).Debug("Starting sampler")
		sr := sampler.StartSamplerRoutine(t, s.sampleQueue)
		samplerRoutines = append(samplerRoutines, sr)
	}

	for {
		select {
		case samples := <-s.sampleQueue:
			now := time.Now().Unix()
			for _, e := range samples {
				e.Timestamp(now)
				s.ctx.SendEvent(e, "")
			}

		case <-s.stopChannel:
			// Stop channel has been closed - exit.
			for _, sr := range samplerRoutines {
				sr.Stop()
			}
			return
		}
	}
}
