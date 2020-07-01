// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextCache(t *testing.T) {
	// setup fake code
	now := time.Now()
	clock := func() time.Time {
		return now
	}
	value := "hello"
	minFetch := func() ([]discovery.Discovery, error) {
		disc := NewDiscovery(data.Map{"minute": value}, nil, nil)
		return []discovery.Discovery{disc}, nil
	}
	hourFetch := func() (interface{}, error) { return map[string]string{"value": value}, nil }
	type fetched struct{ Minute, Hour, Hour5 string }

	// GIVEN a context fetcher with cache configurations for 1 minute, 1 hour and 5 hours
	ctx := Sources{
		clock: clock,
		discoverer: &discoverer{
			cache: cachedEntry{ttl: time.Minute},
			fetch: minFetch,
		},
		variables: map[string]*gatherer{
			"hour": {
				cache: cachedEntry{ttl: time.Hour},
				fetch: hourFetch,
			},
			"hour5": {
				cache: cachedEntry{ttl: 5 * time.Hour},
				fetch: hourFetch,
			},
		},
	}

	fetch := func() fetched {
		b := New()
		vals, err := b.Fetch(&ctx)
		require.NoError(t, err)
		matches, err := b.Replace(&vals, fetched{"${minute}", "${hour.value}", "${hour5.value}"})
		require.NoError(t, err)

		require.Len(t, matches, 1)
		require.IsType(t, fetched{}, matches[0].Variables)
		return matches[0].Variables.(fetched)

	}
	// WHEN the data is fetched for the first time
	result := fetch()
	// THEN all the values are updated
	assert.Equal(t, fetched{"hello", "hello", "hello"}, result)

	// AND when the data is fetched again after the ttls expire
	value = "newValue"
	now = now.Add(5 * time.Second)
	result = fetch()
	// THEN no values are updated
	assert.Equal(t, fetched{"hello", "hello", "hello"}, result)

	// AND when the 1-minute ttl expires
	now = now.Add(60 * time.Second)
	// THEN the minute value is updated
	result = fetch()
	assert.Equal(t, fetched{"newValue", "hello", "hello"}, result)

	// AND when the 1-hour ttl expires
	now = now.Add(time.Hour)
	value = "anotherValue"
	// THEN the 1-hour (and minute) value is updated
	result = fetch()
	assert.Equal(t, fetched{"anotherValue", "anotherValue", "hello"}, result)

	// AND when the 5-hour ttl expires
	now = now.Add(5 * time.Hour)
	value = "bye"
	// THEN the all the values are updated
	result = fetch()
	assert.Equal(t, fetched{"bye", "bye", "bye"}, result)

	// AND if data is queried immediately after
	now = now.Add(5 * time.Second)
	value = "this won't be fetched!"
	// THEN no values have expired and not updated
	result = fetch()
	assert.Equal(t, fetched{"bye", "bye", "bye"}, result)
}
