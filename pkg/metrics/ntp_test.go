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

	timeout := uint(100)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ntp := NewNtp(testCase.pool, timeout, testCase.interval)
			assert.Equal(t, testCase.expectedInterval, ntp.interval)
			assert.Equal(t, testCase.expectedPool, ntp.pool)
			assert.Equal(t, time.Duration(timeout)*time.Second, ntp.timeout)
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
			expectedTimeout: time.Duration(125) * time.Second,
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

func TestOffset_Interval(t *testing.T) {
	testCases := []struct {
		name              string
		interval          uint
		timeout           uint
		offset            time.Duration
		pool              []string
		now               func() time.Time
		ntpQuery          func(host string, opt ntp.QueryOptions) (*ntp.Response, error)
		ntpResponses      []ntpResp
		updatedAt         time.Time
		expectedOffset    time.Duration
		expectedError     error
		expectedUpdatedAt time.Time
	}{
		{
			name:          "empty list of hosts should return error",
			expectedError: ErrEmptyNtpHosts,
		},
		{
			name:              "first request should not use cached value",
			now:               nowMock("2022-09-28 16:02:45"),
			pool:              []string{"one", "two"},
			ntpResponses:      []ntpResp{{buildValidNtpResponse(50 * time.Millisecond), nil}, {buildValidNtpResponse(10 * time.Millisecond), nil}},
			expectedOffset:    30 * time.Millisecond,
			expectedUpdatedAt: mustParse("2006-01-02 15:04:05", "2022-09-28 16:02:45"), // same as now
		},
		{
			name:              "request before interval should return interval error",
			now:               nowMock("2022-09-28 16:02:45"),
			updatedAt:         mustParse("2006-01-02 15:04:05", "2022-09-28 15:52:45"),
			pool:              []string{"one", "two"},
			expectedError:     ErrNotInInterval,
			expectedUpdatedAt: mustParse("2006-01-02 15:04:05", "2022-09-28 15:52:45"),
		},
		{
			name:              "request after interval should request new value",
			now:               nowMock("2022-09-28 16:02:45"),
			updatedAt:         mustParse("2006-01-02 15:04:05", "2022-09-28 15:42:45"),
			pool:              []string{"one", "two"},
			offset:            20 * time.Millisecond,
			ntpResponses:      []ntpResp{{buildValidNtpResponse(50 * time.Millisecond), nil}, {buildValidNtpResponse(10 * time.Millisecond), nil}},
			expectedOffset:    30 * time.Millisecond,
			expectedUpdatedAt: mustParse("2006-01-02 15:04:05", "2022-09-28 16:02:45"), // same as now
		},
		{
			name:              "request with query error should update updatedAt",
			now:               nowMock("2022-09-28 16:02:45"),
			updatedAt:         mustParse("2006-01-02 15:04:05", "2022-09-28 15:42:45"),
			pool:              []string{"one"},
			ntpResponses:      []ntpResp{{nil, errors.New("this is an error")}},
			expectedError:     ErrGettingNtpOffset,
			expectedUpdatedAt: mustParse("2006-01-02 15:04:05", "2022-09-28 16:02:45"), // same as now
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ntpMonitor := NewNtp(testCase.pool, testCase.timeout, testCase.interval)
			ntpMonitor.ntpQuery = ntpQueryMock(testCase.ntpResponses...)
			ntpMonitor.now = testCase.now
			ntpMonitor.updatedAt = testCase.updatedAt
			offset, err := ntpMonitor.Offset()
			assert.Equal(t, testCase.expectedOffset, offset)
			assert.ErrorIs(t, err, testCase.expectedError)
			assert.Equal(t, testCase.expectedUpdatedAt, ntpMonitor.updatedAt)
		})
	}
}

func TestOffset_OffsetAverage(t *testing.T) {
	timeout := uint(5000)
	interval := uint(15)
	ntpMonitor := NewNtp([]string{"one", "two", "three"}, timeout, interval)
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{
		{
			resp: buildValidNtpResponse(110 * time.Millisecond),
			err:  nil,
		},
		{
			resp: buildValidNtpResponse(20 * time.Millisecond),
			err:  nil,
		},
		{
			resp: buildValidNtpResponse(23 * time.Millisecond),
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
			resp: buildValidNtpResponse(50 * time.Millisecond),
			err:  nil,
		},
		{
			err: errors.New("this is an error"),
		},
		{
			resp: buildValidNtpResponse(40 * time.Millisecond),
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
			err: errors.New("this is an error1"),
		},
		{
			err: errors.New("this is an error2"),
		},
		{
			err: errors.New("this is an error3"),
		},
	}...)
	ntpMonitor.now = nowMock("2022-09-28 16:02:45")
	offset, err := ntpMonitor.Offset()
	assert.Equal(t, time.Duration(0), offset)
	assert.ErrorAs(t, err, &ErrGettingNtpOffset)
}

func TestOffset_InvalidNtpResponse(t *testing.T) {
	t.Parallel()

	timeout := uint(5000)
	interval := uint(15)
	ntpMonitor := NewNtp([]string{"one", "two", "three"}, timeout, interval)
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{
		{
			resp: buildInvalidNtpResponse(),
			err:  nil,
		},
		{
			resp: buildInvalidNtpResponse(),
			err:  nil,
		},
		{
			resp: buildInvalidNtpResponse(),
			err:  nil,
		},
	}...)
	ntpMonitor.now = nowMock("2022-09-28 16:02:45")
	offset, err := ntpMonitor.Offset()
	assert.Equal(t, time.Duration(0), offset)
	assert.ErrorAs(t, err, &ErrGettingNtpOffset)
}

func TestOffset_AnyHostInvalidShouldNotReturnError(t *testing.T) {
	t.Parallel()

	timeout := uint(5000)
	interval := uint(15)
	ntpMonitor := NewNtp([]string{"one", "two", "three"}, timeout, interval)
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{
		{
			resp: buildValidNtpResponse(50 * time.Millisecond),
			err:  nil,
		},
		{
			resp: buildInvalidNtpResponse(),
			err:  nil,
		},
		{
			resp: buildValidNtpResponse(40 * time.Millisecond),
			err:  nil,
		},
	}...)
	ntpMonitor.now = nowMock("2022-09-28 16:02:45")
	offset, err := ntpMonitor.Offset()
	assert.Equal(t, time.Millisecond*45, offset)
	assert.Equal(t, nil, err)
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

func buildValidNtpResponse(offset time.Duration) *ntp.Response {
	// A response that should pass the ntp.Validate() check without errors
	return &ntp.Response{ //nolint:exhaustruct
		// Stratum should be not 0 and lower than 16
		Stratum: 1,
		// Leap should not be ntp.LeapNotInSync
		Leap: ntp.LeapNoWarning,

		ClockOffset: offset,
	}
}

func buildInvalidNtpResponse() *ntp.Response {
	// A response that should fail the ntp.Validate() check
	return &ntp.Response{ //nolint:exhaustruct
		// Stratum should be not 0 and lower than 16
		Stratum: 0,
		// Leap should not be ntp.LeapNotInSync
		Leap: ntp.LeapNotInSync,
	}
}
