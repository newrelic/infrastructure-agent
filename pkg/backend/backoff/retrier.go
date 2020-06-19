// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package backoff

import "time"

// RetryManager retries using 2 different approaches:
// - fixed time using SetNextRetryAfter
// - exponential backoff using SetNextRetryWithBackoff
type RetryManager struct {
	retryStarted time.Time
	retryDelay   time.Duration
	retryBackoff *Backoff
}

// NewRetrier creates a new retrier
func NewRetrier() *RetryManager {
	return &RetryManager{}
}

// NewRetrierWithBackoff creates a new retrier with a specific backoff
func NewRetrierWithBackoff(backoff *Backoff) *RetryManager {
	return &RetryManager{retryBackoff: backoff}
}

// SetNextRetryAfter : sets a provided time to wait
func (m *RetryManager) SetNextRetryAfter(duration time.Duration) {
	m.retryStarted = time.Now()
	m.retryDelay = duration
	m.retryBackoff = nil
}

// SetNextRetryWithBackoff sets a backoff for next retry that when called 2nd time it'll be increase exponentially
func (m *RetryManager) SetNextRetryWithBackoff() {
	m.retryStarted = time.Now()

	if m.retryBackoff == nil {
		m.retryBackoff = NewDefaultBackoff()
	}

	m.retryDelay = m.retryBackoff.Duration()
}

// RetryAfter returns time to wait until next retry
func (m *RetryManager) RetryAfter() time.Duration {
	delay := m.retryDelay

	a := m.retryStarted.Add(delay)
	return a.Sub(time.Now())
}
