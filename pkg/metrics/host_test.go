// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHostSample_CachedNtpOffset(t *testing.T) {
	timeout := uint(5)
	interval := uint(15)
	ntpMonitor := NewNtp([]string{"one"}, timeout, interval)
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{
		{
			resp: buildValidNtpResponse(50 * time.Millisecond),
			err:  nil,
		},
	}...)

	hostMonitor := NewHostMonitor(ntpMonitor)

	expectedOffset := (50 * time.Millisecond).Seconds()
	expectedNtpSample := &expectedOffset

	// valid ntp response
	ntpMonitor.ntpQuery = ntpQueryMock([]ntpResp{{buildValidNtpResponse(50 * time.Millisecond), nil}}...)
	sample, err := hostMonitor.Sample()
	assert.Nil(t, err)
	assert.Equal(t, expectedNtpSample, sample.NtpOffset)

	// query inside interval should return cached value
	sample, err = hostMonitor.Sample()
	assert.Nil(t, err)
	assert.Equal(t, expectedNtpSample, sample.NtpOffset)
}
