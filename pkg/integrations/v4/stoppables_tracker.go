// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v4

import (
	"context"
	"sync"
)

// stoppablesTracker tracks cancellation funcs for integrations that might be requested to be
// stopped later on. Use case: command-api requests to Stop integration by name+args (hash).
type stoppablesTracker struct {
	sync.RWMutex
	hash2Cancel map[string]context.CancelFunc // indexed cancellation for a running integration
}

func newStoppablesTracker() *stoppablesTracker {
	return &stoppablesTracker{
		hash2Cancel: make(map[string]context.CancelFunc),
	}
}

func (t *stoppablesTracker) Add(ctx context.Context, hash string) context.Context {
	if hash == "" {
		return ctx
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	t.Lock()
	t.hash2Cancel[hash] = cancel
	t.Unlock()

	return ctx
}

func (t *stoppablesTracker) Remove(hash string) {
	if hash == "" {
		return
	}

	t.Lock()
	delete(t.hash2Cancel, hash)
	t.Unlock()
}

func (t *stoppablesTracker) Stop(hash string) (stopped bool) {
	t.RLock()
	cancel, ok := t.hash2Cancel[hash]
	t.RUnlock()
	if ok {
		cancel()
		stopped = true
		t.Remove(hash)
	}

	return
}
