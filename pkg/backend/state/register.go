// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package state

// State values and getter could be private if others components don't need to know current state

import (
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
)

// GetIDs entity returned state
type Register int

// GetIDs endpoint states
const (
	RegisterHealthy Register = iota
	RegisterRetryAfter
	RegisterRetryBackoff
)

func (r Register) String() string {
	switch r {
	case RegisterHealthy:
		return "RegisterHealthy"
	case RegisterRetryAfter:
		return "RegisterRetryAfter"
	case RegisterRetryBackoff:
		return "RegisterRetryBackoff"
	}
	return "Unknown"
}

// RegisterSM register endpoint state manager
type RegisterSM interface {
	// state getters
	State() Register
	RetryAfter() time.Duration
	// setters
	NextRetryAfter(duration time.Duration)
	NextRetryWithBackoff()
}

// NewRegisterSM creates a new register endpoint state manager
func NewRegisterSM() RegisterSM {
	return &registerManager{
		mutex:   sync.Mutex{},
		state:   RegisterHealthy,
		retrier: backoff.NewRetrier(),
	}
}

type registerManager struct {
	mutex   sync.Mutex
	state   Register
	retrier *backoff.RetryManager
}

func (m *registerManager) State() Register {
	if m.state != RegisterHealthy && m.RetryAfter() <= 0 {
		m.state = RegisterHealthy
	}

	return m.state
}

func (m *registerManager) RetryAfter() time.Duration {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.retrier.RetryAfter()
}

func (m *registerManager) NextRetryAfter(duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.retrier.SetNextRetryAfter(duration)
	m.state = RegisterRetryAfter
}

func (m *registerManager) NextRetryWithBackoff() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.retrier.SetNextRetryWithBackoff()
	m.state = RegisterRetryBackoff
}
