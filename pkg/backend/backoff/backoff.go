// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package backoff provides an exponential-backoff implementation.
// https://github.com/jpillora/backoff inlined for customizations.
package backoff

import (
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"math"
	"math/rand"
	"time"
)

// Backoff is a time.Duration counter, starting at Min. After every call to
// the Duration method the current timing is multiplied by Factor, but it
// never exceeds Max.
//
// Backoff is not generally concurrent-safe, but the ForAttempt method can
// be used concurrently.
type Backoff struct {
	//Factor is the multiplying factor for each increment step
	attempt, Factor float64
	//Jitter eases contention by randomizing backoff steps
	Jitter bool
	//Min and Max are the minimum and maximum values of the counter
	Min, Max time.Duration
}

// Default values
const (
	DefaultFactor = 2
	DefaultJitter = true
	DefaultMin    = 1 * time.Second
	DefaultMax    = 5 * time.Minute
)

// NewDefaultBackoff default behaviour for Vortex.
func NewDefaultBackoff() *Backoff {
	return &Backoff{
		Factor: DefaultFactor,
		Jitter: DefaultJitter,
		Min:    DefaultMin,
		Max:    DefaultMax,
	}
}

// Duration returns the duration for the current attempt. The result will be limited to max value.
func (b *Backoff) DurationWithMax(max time.Duration) time.Duration {
	if max <= 0 {
		max = b.Max
	}
	return b.duration(b.Min, max)
}

// Duration returns the duration for the current attempt.
func (b *Backoff) Duration() time.Duration {
	return b.duration(b.Min, b.Max)
}

// duration returns the duration for the current attempt before incrementing
// the attempt counter. See ForAttempt.
func (b *Backoff) duration(min, max time.Duration) time.Duration {
	d := b.forAttempt(b.attempt, min, max)
	b.attempt++
	return d
}

const maxInt64 = float64(math.MaxInt64 - 512)

// ForAttempt calls forAttempt with configured max/min values.
func (b *Backoff) ForAttempt(attempt float64) time.Duration {
	return b.forAttempt(attempt, b.Min, b.Max)
}

// forAttempt returns the duration for a specific attempt. This is useful if
// you have a large number of independent Backoffs, but don't want use
// unnecessary memory storing the Backoff parameters per Backoff. The first
// attempt should be 0.
//
// forAttempt is concurrent-safe.
func (b *Backoff) forAttempt(attempt float64, min, max time.Duration) time.Duration {
	// Zero-values are nonsensical, so we use
	// them to apply defaults
	if min <= 0 {
		min = 100 * time.Millisecond
	}
	if max <= 0 {
		max = 10 * time.Second
	}
	if min >= max {
		// short-circuit
		return max
	}
	factor := b.Factor
	if factor <= 0 {
		factor = 2
	}
	//calculate this duration
	minf := float64(min)
	durf := minf * math.Pow(factor, attempt)
	if b.Jitter {
		durf = rand.Float64()*(durf-minf) + minf
	}
	//ensure float64 wont overflow int64
	if durf > maxInt64 {
		return max
	}
	dur := time.Duration(durf)
	//keep within bounds
	if dur < min {
		return min
	}
	if dur > max {
		return max
	}
	return dur
}

// Reset restarts the current attempt counter at zero.
func (b *Backoff) Reset() {
	b.attempt = 0
}

// Attempt returns the current attempt counter value.
func (b *Backoff) Attempt() float64 {
	return b.attempt
}

// Copy returns a backoff with equals constraints as the original
func (b *Backoff) Copy() *Backoff {
	return &Backoff{
		Factor: b.Factor,
		Jitter: b.Jitter,
		Min:    b.Min,
		Max:    b.Max,
	}
}

// GetMaxBackoffByCause will return the maximum backoff value based on the error cause.
func GetMaxBackoffByCause(cause backendhttp.ErrorCause) time.Duration {
	switch cause {
	case backendhttp.InvalidLicense, backendhttp.TrialExpired:
		return 1 * time.Hour
	case backendhttp.TrialInactive, backendhttp.ServiceError:
		return 5 * time.Minute
	default:
		return 0
	}
}
