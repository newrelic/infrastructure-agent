// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package stoppable

import (
	"context"
	"sync"
)

// Tracker tracks PIDs and cancellation funcs for integrations that might be requested to be stopped
// later on. Use case: command-api requests to stop an integration identified by hash (name+args).
type Tracker struct {
	lock        sync.RWMutex
	hash2Cancel map[string]pidChanNCancel // indexed cancellation & pid receiver for an integration
}

type pidChanNCancel struct {
	pidC   chan int
	cancel context.CancelFunc
}

func NewTracker() *Tracker {
	return &Tracker{
		hash2Cancel: make(map[string]pidChanNCancel),
	}
}

func (t *Tracker) Track(ctx context.Context, hash string) (context.Context, chan<- int) {
	if hash == "" {
		return ctx, nil
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	t.lock.Lock()
	pidC := make(chan int, 1)
	t.hash2Cancel[hash] = pidChanNCancel{
		cancel: cancel,
		pidC:   pidC,
	}
	t.lock.Unlock()

	return ctx, pidC
}

func (t *Tracker) Untrack(hash string) {
	if hash == "" {
		return
	}

	t.lock.Lock()
	delete(t.hash2Cancel, hash)
	t.lock.Unlock()
}

// PIDReadChan returns a PID receiver channel of an stoppable process, nil is returned when not tracked.
func (t *Tracker) PIDReadChan(hash string) (pidReadChan <-chan int, tracked bool) {
	cancel, tracked := t.hash2Cancel[hash]
	if tracked {
		return cancel.pidC, tracked
	}
	return nil, false
}

// Kill cancels context, for a running process it'll be SIGKILLed.
func (t *Tracker) Kill(hash string) (stopped bool) {
	t.lock.RLock()
	cancel, ok := t.hash2Cancel[hash]
	t.lock.RUnlock()
	if ok {
		cancel.cancel()
		stopped = true
		t.Untrack(hash)
	}

	return
}
