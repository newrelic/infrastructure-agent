// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"testing"
	"time"

	"go.uber.org/multierr"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/secrets"
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

func mockGatherer(ttl time.Duration, data interface{}) *gatherer {
	return &gatherer{
		cache: cachedEntry{ttl: ttl}, //nolint:exhaustruct
		fetch: func() (interface{}, error) {
			return data, nil
		},
	}
}

// dataWithTTL is a custom implementation of a payload exposing a TTL.
type dataWithTTL map[string]interface{}

func (ttl dataWithTTL) TTL() (time.Duration, error) {
	var ok bool //nolint:varnamelen
	var ttlData string

	if _, ok = ttl["ttl"]; !ok {
		return 0, secrets.ErrTTLNotFound
	}

	if ttlData, ok = ttl["ttl"].(string); !ok {
		return 0, secrets.ErrTTLNotFound
	}

	t, err := time.ParseDuration(ttlData)
	if err != nil {
		return 0, multierr.Append(secrets.ErrTTLInvalid, err)
	}

	return t, nil
}

func (ttl dataWithTTL) Data() (map[string]interface{}, error) {
	if _, ok := ttl["data"]; !ok {
		return nil, ErrDataNotFound
	}

	if _, ok := ttl["data"].(map[string]interface{}); !ok {
		return nil, ErrDataInvalid
	}

	//nolint:forcetypeassert
	return ttl["data"].(map[string]interface{}), nil
}

func Test_GathererCacheTtlFromPayload(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name               string
		cacheInitialTTL    time.Duration
		mockData           interface{}
		expectedTTLInCache time.Duration
		expectedError      error
	}{
		{
			name:            "no ttl implementation should respect original ttl",
			cacheInitialTTL: time.Second * 35,
			mockData: map[string]interface{}{
				"ttl": "12s",
			},
			expectedTTLInCache: time.Second * 35,
		},
		{
			name:            "ttl implementation should override original ttl",
			cacheInitialTTL: time.Second * 35,
			mockData: dataWithTTL{
				"ttl":  "12s",
				"data": map[string]interface{}{"some data": "in a map"},
			},
			expectedTTLInCache: time.Second * 12,
		},
		{
			name:            "invalid ttl should fallback to default ttl",
			cacheInitialTTL: time.Second * 35,
			mockData: dataWithTTL{
				"ttl":  "invalid duration",
				"data": map[string]interface{}{"some data": "in a map"},
			},
			expectedTTLInCache: defaultVariablesTTL,
		},
		{
			name:            "no ttl shoul fallback to default ttl",
			cacheInitialTTL: time.Second * 35,
			mockData: dataWithTTL{
				"data": map[string]interface{}{"some data": "in a map"},
			},
			expectedTTLInCache: defaultVariablesTTL,
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			gat := mockGatherer(testCase.cacheInitialTTL, testCase.mockData)
			source := Sources{ //nolint:exhaustruct
				clock: time.Now,
				variables: map[string]*gatherer{
					"aws-kms": gat,
				},
			}
			_, err := Fetch(&source)
			if testCase.expectedError != nil {
				assert.ErrorAs(t, err, &testCase.expectedError)
			}
			assert.Equal(t, testCase.expectedTTLInCache, gat.cache.ttl)
		})
	}
}

//nolint:funlen
func TestTtlE2E(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description   string
		yaml          string
		fetch         func() (interface{}, error)
		expectedTTL   time.Duration
		expectedKey   string
		expectedValue string
	}{
		{
			description: "no TTL defaults to defaultVariablesTTL",
			yaml: `
variables:
  myData:
    obfuscated:
      key: is not used in the test
      secret: is not used in the test
`,
			expectedTTL:   defaultVariablesTTL,
			expectedKey:   "myData.data",
			expectedValue: "some_value",
			fetch: func() (interface{}, error) {
				return map[string]string{
					"data": "some_value",
				}, nil
			},
		},
		{
			description: "TTL in conf overrides defaults",
			yaml: `
variables:
  myData:
    ttl: 345s
    obfuscated:
      key: is not used in the test
      secret: is not used in the test
`,
			expectedTTL:   time.Second * 345,
			expectedKey:   "myData.data",
			expectedValue: "some_value",
			fetch: func() (interface{}, error) {
				return map[string]string{
					"data": "some_value",
				}, nil
			},
		},
		{
			description: "TTL with no implementation has no efect in ttl",
			yaml: `
variables:
  myData:
    obfuscated:
      key: is not used in the test
      secret: is not used in the test
`,
			expectedTTL:   defaultVariablesTTL,
			expectedKey:   "myData.data",
			expectedValue: "some_value",
			fetch: func() (interface{}, error) {
				return map[string]string{
					"data": "some_value",
					"ttl":  "1432s",
				}, nil
			},
		},
		{
			description: "TTL with implementation overrides default ttl",
			yaml: `
variables:
  myData:
    obfuscated:
      key: is not used in the test
      secret: is not used in the test
`,
			expectedTTL:   time.Second * 1432,
			expectedKey:   "myData.some_data",
			expectedValue: "in a map",
			fetch: func() (interface{}, error) {
				return dataWithTTL{
					"data": map[string]interface{}{"some_data": "in a map"},
					"ttl":  "1432s",
				}, nil
			},
		},
		{
			description: "TTL with implementation overrides conf ttl",
			yaml: `
variables:
  myData:
    ttl: 345s
    obfuscated:
      key: is not used in the test
      secret: is not used in the test
`,
			expectedTTL:   time.Second * 1432,
			expectedKey:   "myData.some_data",
			expectedValue: "in a map",
			fetch: func() (interface{}, error) {
				return dataWithTTL{
					"data": map[string]interface{}{"some_data": "in a map"},
					"ttl":  "1432s",
				}, nil
			},
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			sources, err := LoadYAML([]byte(testCase.yaml))
			assert.NoError(t, err)
			sources.clock = time.Now
			sources.variables["myData"].fetch = testCase.fetch

			values, err := Fetch(sources)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedValue, values.vars[testCase.expectedKey])
			assert.Equal(t, testCase.expectedTTL, sources.variables["myData"].cache.ttl)
		})
	}
}
