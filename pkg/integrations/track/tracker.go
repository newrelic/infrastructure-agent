// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package track

import (
	"context"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// Tracker tracks integrations context while running so actions can be taken later on based on those.
// Integrations are identified by hash: name+args.
// Use cases:
// - command-api requests to stop an integration, for which PID and ctx cancellation are stored.
// - integrations run by command-api should notify their exit-code by submiting an event.
// In case we need to cover more cases, it'd be nice to decouple cases logic from the tracking one.
type Tracker struct {
	lock         sync.RWMutex
	hash2Ctx     map[string]integrationContext // indexed cancellation & pid receiver for an integration
	eventEmitter dm.Emitter
}

type integrationContext struct {
	pidC   chan int
	cancel context.CancelFunc
	def    integration.Definition
}

// NewTracker creates new integrations tracker. If an "emitter" is provided then an event will be
// submited containing the exit code of the integration execution.
func NewTracker(emitter dm.Emitter) *Tracker {
	return &Tracker{
		hash2Ctx:     make(map[string]integrationContext),
		eventEmitter: emitter,
	}
}

// Track tracks (indexing by hash) an integration run alogn with its context and optionally
// Definition. When Definition is not provided NotifyExit method won't take any effect.
func (t *Tracker) Track(ctx context.Context, hash string, def *integration.Definition) (context.Context, chan<- int) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	pidC := make(chan int, 1)
	var defNotNil integration.Definition
	if def != nil {
		defNotNil = *def
	}
	t.lock.Lock()
	t.hash2Ctx[hash] = integrationContext{
		cancel: cancel,
		pidC:   pidC,
		def:    defNotNil,
	}
	t.lock.Unlock()

	return ctx, pidC
}

func (t *Tracker) Untrack(hash string) {
	if hash == "" {
		return
	}

	t.lock.Lock()
	delete(t.hash2Ctx, hash)
	t.lock.Unlock()
}

// PIDReadChan returns a PID receiver channel of an stoppable process, nil is returned when not tracked.
func (t *Tracker) PIDReadChan(hash string) (pidReadChan <-chan int, tracked bool) {
	t.lock.RLock()
	cancel, tracked := t.hash2Ctx[hash]
	t.lock.RUnlock()
	if tracked {
		return cancel.pidC, tracked
	}
	return nil, false
}

// Kill cancels context, for a running process it'll be SIGKILLed.
func (t *Tracker) Kill(hash string) (stopped bool) {
	t.lock.RLock()
	cancel, ok := t.hash2Ctx[hash]
	t.lock.RUnlock()
	if ok {
		cancel.cancel()
		stopped = true
		t.Untrack(hash)
	}

	return
}

func (t *Tracker) NotifyExit(hash string, exitCode int) {
	if t.eventEmitter == nil {
		return
	}

	ts := time.Now().UnixNano()

	t.lock.RLock()
	iCtx, tracked := t.hash2Ctx[hash]
	t.lock.RUnlock()

	if !tracked || iCtx.def.CmdChanReq == nil {
		return
	}

	ev := iCtx.def.CmdChanReq.Event("integration-exited")
	ev["cmd_exit_code"] = exitCode

	ds := protocol.NewEventDataset(ts, ev)
	data := protocol.NewData("tracker.notifyexit", "1", []protocol.Dataset{ds})
	t.eventEmitter.Send(fwrequest.NewFwRequest(iCtx.def, nil, nil, data))
}
