// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package backoff

import (
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"

	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test1(t *testing.T) {

	b := &Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
	}

	equals(t, b.Duration(), 100*time.Millisecond)
	equals(t, b.Duration(), 200*time.Millisecond)
	equals(t, b.Duration(), 400*time.Millisecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Millisecond)
}

func TestForAttempt(t *testing.T) {

	b := &Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
	}

	equals(t, b.ForAttempt(0), 100*time.Millisecond)
	equals(t, b.ForAttempt(1), 200*time.Millisecond)
	equals(t, b.ForAttempt(2), 400*time.Millisecond)
	b.Reset()
	equals(t, b.ForAttempt(0), 100*time.Millisecond)
}

func Test2(t *testing.T) {

	b := &Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 1.5,
	}

	equals(t, b.Duration(), 100*time.Millisecond)
	equals(t, b.Duration(), 150*time.Millisecond)
	equals(t, b.Duration(), 225*time.Millisecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Millisecond)
}

func Test3(t *testing.T) {

	b := &Backoff{
		Min:    100 * time.Nanosecond,
		Max:    10 * time.Second,
		Factor: 1.75,
	}

	equals(t, b.Duration(), 100*time.Nanosecond)
	equals(t, b.Duration(), 175*time.Nanosecond)
	equals(t, b.Duration(), 306*time.Nanosecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Nanosecond)
}

func Test4(t *testing.T) {
	b := &Backoff{
		Min:    500 * time.Second,
		Max:    100 * time.Second,
		Factor: 1,
	}

	equals(t, b.Duration(), b.Max)
}

func TestGetAttempt(t *testing.T) {
	b := &Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
	}
	equals(t, b.Attempt(), float64(0))
	equals(t, b.Duration(), 100*time.Millisecond)
	equals(t, b.Attempt(), float64(1))
	equals(t, b.Duration(), 200*time.Millisecond)
	equals(t, b.Attempt(), float64(2))
	equals(t, b.Duration(), 400*time.Millisecond)
	equals(t, b.Attempt(), float64(3))
	b.Reset()
	equals(t, b.Attempt(), float64(0))
	equals(t, b.Duration(), 100*time.Millisecond)
	equals(t, b.Attempt(), float64(1))
}

func TestJitter(t *testing.T) {
	b := &Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	equals(t, b.Duration(), 100*time.Millisecond)
	between(t, b.Duration(), 100*time.Millisecond, 200*time.Millisecond)
	between(t, b.Duration(), 100*time.Millisecond, 400*time.Millisecond)
	b.Reset()
	equals(t, b.Duration(), 100*time.Millisecond)
}

func TestCopy(t *testing.T) {
	b := &Backoff{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
	}
	b2 := b.Copy()
	equals(t, b, b2)
}

func between(t *testing.T, actual, low, high time.Duration) {
	if actual < low {
		t.Fatalf("Got %s, Expecting >= %s", actual, low)
	}
	if actual > high {
		t.Fatalf("Got %s, Expecting <= %s", actual, high)
	}
}

func equals(t *testing.T, v1, v2 interface{}) {
	if !reflect.DeepEqual(v1, v2) {
		t.Fatalf("Got %v, Expecting %v", v1, v2)
	}
}

func TestDurationWithMax(t *testing.T) {
	b := NewDefaultBackoff()
	b.Jitter = false
	b.Min = 1 * time.Nanosecond

	max := 4 * time.Nanosecond

	assert.Equal(t, 1*time.Nanosecond, b.DurationWithMax(max))
	assert.Equal(t, 2*time.Nanosecond, b.DurationWithMax(max))
	assert.Equal(t, max, b.DurationWithMax(max))
	assert.Equal(t, max, b.DurationWithMax(max), "at this point max was reached")
}

func TestGetMaxBackoffByCause(t *testing.T) {
	serviceErrorBackoff := GetMaxBackoffByCause(backendhttp.ServiceError)
	assert.Equal(t, 5*time.Minute, serviceErrorBackoff)

	trialInactiveBackoff := GetMaxBackoffByCause(backendhttp.TrialInactive)
	assert.Equal(t, 5*time.Minute, trialInactiveBackoff)

	invalidLicenseBackoff := GetMaxBackoffByCause(backendhttp.InvalidLicense)
	assert.Equal(t, 1*time.Hour, invalidLicenseBackoff)

	trialExpiredBackoff := GetMaxBackoffByCause(backendhttp.TrialExpired)
	assert.Equal(t, 1*time.Hour, trialExpiredBackoff)
}
