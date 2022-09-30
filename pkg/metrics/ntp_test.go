/*
 *
 *  * Copyright 2021 New Relic Corporation. All rights reserved.
 *  * SPDX-License-Identifier: Apache-2.0
 *
 */

package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/beevik/ntp"
	"github.com/stretchr/testify/assert"
)

func TestNewNtp_Interval(t *testing.T) {
	testCases := []struct {
		name             string
		interval         uint
		pool             []string
		expectedInterval time.Duration
		expectedPool     []string
	}{
		{
			name:             "invalid interval fallbacks to minimum",
			interval:         2,
			expectedInterval: time.Duration(ntpIntervalMin) * time.Minute,
		},
		{
			name:             "pool is allowed to be empty",
			expectedInterval: time.Duration(ntpIntervalMin) * time.Minute,
		},
		{
			name:             "valid interval",
			interval:         17,
			expectedInterval: time.Duration(17) * time.Minute,
		},
		{
			name:             "valid pool",
			pool:             []string{"one", "two", "three"},
			expectedPool:     []string{"one", "two", "three"},
			expectedInterval: time.Duration(ntpIntervalMin) * time.Minute,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ntp := NewNtp(testCase.pool, 100, testCase.interval)
			assert.Equal(t, testCase.expectedInterval, ntp.interval)
			assert.Equal(t, testCase.expectedPool, ntp.pool)
			assert.Equal(t, 100*time.Minute, ntp.timeout)
		})
	}
}

func TestNewNtp_Timeout(t *testing.T) {
	testCases := []struct {
		name            string
		timeout         uint
		pool            []string
		expectedTimeout time.Duration
	}{
		{
			name:            "valid timeout",
			timeout:         125,
			expectedTimeout: time.Duration(125) * time.Millisecond,
		},
		{
			name:            "no timeout shoulid default to 5secs",
			expectedTimeout: time.Duration(5) * time.Second,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ntp := NewNtp([]string{"one", "two", "three"}, testCase.timeout, 1000)
			assert.Equal(t, []string{"one", "two", "three"}, ntp.pool)
			assert.Equal(t, 1000*time.Minute, ntp.interval)
			assert.Equal(t, testCase.expectedTimeout, ntp.timeout)
		})
	}
}

func TestOffset_Cache(t *testing.T) {
	testCases := []struct {
		name           string
		interval       uint
		timeout        uint
		offset         time.Duration
		pool           []string
		now            func() time.Time
		ntpQuery       func(host string, opt ntp.QueryOptions) (*ntp.Response, error)
		ntpResponses   []ntpResp
		updatedAt      time.Time
		expectedOffset time.Duration
		expectedError  error
	}{
		{
			name:          "empty list of hosts should return error",
			expectedError: ErrEmptyNtpHosts,
		},
		{
			name:           "first request should not use cached value",
			now:            nowMock("2022-09-28 16:02:45"),
			pool:           []string{"one", "two"},
			ntpResponses:   []ntpResp{{&ntp.Response{ClockOffset: 50 * time.Millisecond}, nil}, {&ntp.Response{ClockOffset: 10 * time.Millisecond}, nil}},
			expectedOffset: 30 * time.Millisecond,
		},
		{
			name:           "request before interval should use cached value",
			now:            nowMock("2022-09-28 16:02:45"),
			updatedAt:      mustParse("2006-01-02 15:04:05", "2022-09-28 15:52:45"),
			pool:           []string{"one", "two"},
			offset:         20 * time.Millisecond,
			expectedOffset: 20 * time.Millisecond,
		},
		{
			name:           "request after interval should request new value",
			now:            nowMock("2022-09-28 16:02:45"),
			updatedAt:      mustParse("2006-01-02 15:04:05", "2022-09-28 15:42:45"),
			pool:           []string{"one", "two"},
			offset:         20 * time.Millisecond,
			ntpResponses:   []ntpResp{{&ntp.Response{ClockOffset: 50 * time.Millisecond}, nil}, {&ntp.Response{ClockOffset: 10 * time.Millisecond}, nil}},
			expectedOffset: 30 * time.Millisecond,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ntpMonitor := NewNtp(testCase.pool, testCase.timeout, testCase.interval)
			ntpMonitor.ntpQuery = ntpQueryMock(testCase.ntpResponses...)
			ntpMonitor.now = testCase.now
			ntpMonitor.updatedAt = testCase.updatedAt
			ntpMonitor.offset = testCase.offset
			offset, err := ntpMonitor.Offset()
			assert.Equal(t, testCase.expectedOffset, offset)
			assert.Equal(t, testCase.expectedError, err)
		})
	}
}

func TestOffset_OffsetAverage(t *testing.T) {
	timeout := uint(5000)
	interval := uint(15)
	ntpMonitor := NewNtp([]string{"one", "two", "three"}, timeout, interval)
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{
		{
			resp: &ntp.Response{ClockOffset: 110 * time.Millisecond},
			err:  nil,
		},
		{
			resp: &ntp.Response{ClockOffset: 20 * time.Millisecond},
			err:  nil,
		},
		{
			resp: &ntp.Response{ClockOffset: 23 * time.Millisecond},
			err:  nil,
		},
	}...)
	ntpMonitor.now = nowMock("2022-09-28 16:02:45")
	offset, err := ntpMonitor.Offset()
	assert.Equal(t, time.Millisecond*51, offset)
	assert.Equal(t, nil, err)
}

func TestOffset_AnyHostErrorShouldNotReturnError(t *testing.T) {
	timeout := uint(5000)
	interval := uint(15)
	ntpMonitor := NewNtp([]string{"one", "two", "three"}, timeout, interval)
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{
		{
			resp: &ntp.Response{ClockOffset: 50 * time.Millisecond},
			err:  nil,
		},
		{
			err: errors.New("this is an error"),
		},
		{
			resp: &ntp.Response{ClockOffset: 40 * time.Millisecond},
			err:  nil,
		},
	}...)
	ntpMonitor.now = nowMock("2022-09-28 16:02:45")
	offset, err := ntpMonitor.Offset()
	assert.Equal(t, time.Millisecond*45, offset)
	assert.Equal(t, nil, err)
}

func TestOffset_AllHostErrorShouldReturnError(t *testing.T) {
	timeout := uint(5000)
	interval := uint(15)
	ntpMonitor := NewNtp([]string{"one", "two", "three"}, timeout, interval)
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{
		{
			err: errors.New("this is an error"),
		},
		{
			err: errors.New("this is an error"),
		},
		{
			err: errors.New("this is an error"),
		},
	}...)
	ntpMonitor.now = nowMock("2022-09-28 16:02:45")
	offset, err := ntpMonitor.Offset()
	assert.Equal(t, time.Duration(0), offset)
	assert.ErrorIs(t, ErrGettingNtpOffset, err)
}

// nolint:unparam
func nowMock(dateTime string) func() time.Time {
	return func() time.Time {
		return mustParse("2006-01-02 15:04:05", dateTime)
	}
}

type ntpResp struct {
	resp *ntp.Response
	err  error
}

func ntpQueryMock(resp ...ntpResp) func(host string, opt ntp.QueryOptions) (*ntp.Response, error) {
	idx := 0
	return func(host string, opt ntp.QueryOptions) (*ntp.Response, error) {
		defer func() { idx++ }()
		return resp[idx].resp, resp[idx].err
	}
}

func mustParse(layout string, value string) time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}

	return t
}
