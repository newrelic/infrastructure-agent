// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package backoff

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryAfter_WhenBackoffElapsesReturnsLessThanZero(t *testing.T) {
	r := NewRetrier()
	// short backoff to speed waiting for it to elapse
	r.retryBackoff = &Backoff{
		Factor: 2,
		Min:    100 * time.Millisecond,
		Max:    200 * time.Millisecond,
	}

	// start backoff behaviour triggering next iteration as there's a backoff set
	r.SetNextRetryWithBackoff()

	retryAfter := r.RetryAfter()
	assert.True(t, retryAfter > 0, "retry after: %v should be greater than 0", retryAfter)
	time.Sleep(r.retryBackoff.Max)
	assert.True(t, r.RetryAfter() <= 0)

	r.SetNextRetryWithBackoff()
	assert.True(t, r.RetryAfter() > 0, "retry backoff should have been increased")
}
