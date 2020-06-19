// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package id

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

// Provide identity provider function.
type Provide func() entity.Identity

// UpdateNotifyFn allows to register to listen for ID update events.
type UpdateNotifyFn func(chan<- struct{}, AgentIDNotifyMode)

// AgentIDNotifyMode are flags to define for which notifications the listeners are interested.
type AgentIDNotifyMode uint8

// Contains will check if a AgentIDNotifyMode includes a different AgentIDNotifyMode.
func (a AgentIDNotifyMode) Includes(notifyMode AgentIDNotifyMode) bool {
	return a&notifyMode != 0
}

const (
	// NotifyOnConnect will signal interest for notification when the agent first connect.
	NotifyOnConnect AgentIDNotifyMode = 1 << iota
	// NotifyOnReconnect will signal interest for notification when the agent reconnects.
	NotifyOnReconnect
	// Combine the flags for multiple notifications interests: e.g. NotifyOnAll = NotifyOnConnect | NotifyOnReconnect
)

// Context context for the agent ID.
type Context struct {
	agentIdentity *atomic.Value
	ctx           context.Context
	sem           *sync.Cond
	listeners     map[chan<- struct{}]AgentIDNotifyMode
}

// NewContext creates a new ID context to allow fetching ID in concurrent manner.
func NewContext(ctx context.Context) *Context {
	c := &Context{
		agentIdentity: &atomic.Value{},
		sem:           sync.NewCond(&sync.Mutex{}),
		ctx:           ctx,
		listeners:     make(map[chan<- struct{}]AgentIDNotifyMode),
	}
	c.agentIdentity.Store(entity.EmptyIdentity)
	return c
}

// AgentIDOrEmpty provides agent Identity when available, empty otherwise
func (i *Context) AgentIdnOrEmpty() entity.Identity {
	return i.agentIdentity.Load().(entity.Identity)
}

// AgentID provides a non empty agent ID, blocking until it's available
func (i *Context) AgentID() entity.ID {

	identity := i.agentIdentity.Load().(entity.Identity)
	if !identity.ID.IsEmpty() {
		return identity.ID
	}

	done := make(chan struct{})

	go func() {
		i.sem.L.Lock()
		i.sem.Wait()
		i.sem.L.Unlock()
		close(done)
	}()

	select {
	case <-i.ctx.Done():
		i.sem.Broadcast()
	case <-done:
	}

	return i.agentIdentity.Load().(entity.Identity).ID
}

// AgentIdentity provides agent identity, blocking until connect succeeded (GUID might still be empty).
func (i *Context) AgentIdentity() entity.Identity {
	_ = i.AgentID()

	return i.agentIdentity.Load().(entity.Identity)
}

// SetAgentID sets agent id
func (i *Context) SetAgentIdentity(id entity.Identity) {

	identity := i.agentIdentity.Load().(entity.Identity)
	if !id.ID.IsEmpty() && identity.ID.IsEmpty() {
		i.agentIdentity.Store(id)
		i.sem.Broadcast()
		i.notifyListeners(NotifyOnConnect)
		return
	}

	load := i.agentIdentity.Load()
	shouldNotify := id != load.(entity.Identity)
	i.agentIdentity.Store(id)

	if shouldNotify {
		i.notifyListeners(NotifyOnReconnect)
	}
}

// Notify will register a channel for receiving notifications based on the level of interest.
func (i *Context) Notify(c chan<- struct{}, notifyMode AgentIDNotifyMode) {
	if c == nil {
		return
	}
	i.listeners[c] = notifyMode
}

func (i *Context) notifyListeners(notifyMode AgentIDNotifyMode) {
	for listener, listenerNotifyMode := range i.listeners {
		if !listenerNotifyMode.Includes(notifyMode) {
			continue
		}
		select {
		case listener <- struct{}{}:
		default:
		}
	}
}
