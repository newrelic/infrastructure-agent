// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics_sender

import (
	"fmt"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

const (
	SAMPLE_QUEUE_CAPACITY = 10 // Number of sample batches we'll wait for, min 2 * high freq samplers + 1 * low freq samples
)

// Sender is responsible for submitting data to the collector endpoint.
type Sender struct {
	ctx                  agent.AgentContext
	internalRoutineWaits *sync.WaitGroup // Waitgroup to keep track of how many goroutines are running and wait for them to stop
	stopChannel          chan bool       // Channel will be closed when we want to stop all internal goroutines
	sampleQueue          chan sample.EventBatch
	samplers             []Sampler
}

func NewSender(ctx agent.AgentContext) *Sender {
	return &Sender{
		ctx:                  ctx,
		sampleQueue:          make(chan sample.EventBatch, SAMPLE_QUEUE_CAPACITY),
		internalRoutineWaits: &sync.WaitGroup{},
	}
}

func (self *Sender) RegisterSampler(sampler Sampler) {
	// don't even register the sampler if it's disabled
	if sampler.Disabled() {
		mslog.WithField("sampler", sampler.Name()).Warn("Sampler is disabled and will not run")
		return
	}

	self.samplers = append(self.samplers, sampler)
}

// Start will register the sender with the collector, then start a couple of background
// routines to handle incoming data and post it to the server periodically.
func (self *Sender) Start() (err error) {
	if self.stopChannel != nil {
		return fmt.Errorf("Cannot start sender: The sender is already running. (stopChannel is not nil)")
	}

	// Set up the stop channel so the routines can wait for it to be closed
	self.stopChannel = make(chan bool)

	go func() {
		self.internalRoutineWaits.Add(1)
		self.scheduleSamplers()
		self.internalRoutineWaits.Done()
	}()

	return
}

// Stop will gracefully shut down all sending processes and reset the state of the sender.
// After Stop() returns, it is safe to call Start() again on the same sender instance.
func (self *Sender) Stop() (err error) {
	if self.stopChannel == nil {
		return fmt.Errorf("Cannot stop sender: The sender is not running. (stopChannel is nil)")
	}

	close(self.stopChannel)
	self.internalRoutineWaits.Wait()
	self.stopChannel = nil

	return
}

// Periodically gather all samples and send them to Insights
func (self *Sender) scheduleSamplers() {
	var samplerRoutines []*SamplerRoutine

	for _, sampler := range self.samplers {
		mslog.WithField("sampler", sampler.Name()).Debug("Starting sampler.")
		sr := StartSamplerRoutine(sampler, self.sampleQueue)
		samplerRoutines = append(samplerRoutines, sr)
	}

	for {
		select {
		case samples := <-self.sampleQueue:
			now := time.Now().Unix()
			for _, sample := range samples {
				sample.Timestamp(now)
				self.ctx.SendEvent(sample, "")
			}

		case <-self.stopChannel:
			// Stop channel has been closed - exit.
			for _, sr := range samplerRoutines {
				sr.Stop()
			}
			return
		}
	}
}
