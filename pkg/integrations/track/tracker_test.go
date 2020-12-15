// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package track

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracker_Track(t *testing.T) {
	s := NewTracker(nil)
	ctx, _ := s.Track(context.Background(), "foo", nil)

	select {
	case <-ctx.Done():
		t.Fail()
	default:
	}

	stopped := s.Kill("foo")

	select {
	case <-ctx.Done():
	default:
		t.Fail()
	}

	assert.True(t, stopped)

	_, ok := s.hash2Ctx["foo"]
	assert.False(t, ok, "once stopped context should had been removed from track")
}

func TestTracker_Kill(t *testing.T) {
	s := NewTracker(nil)
	ctx, _ := s.Track(context.Background(), "foo", nil)

	select {
	case <-ctx.Done():
		t.Fail()
	default:
	}

	stopped := s.Kill("bar")

	select {
	case <-ctx.Done():
		t.Fail()
	default:
	}

	assert.False(t, stopped)
}

func TestTracker_PIDReadChan(t *testing.T) {
	s := NewTracker(nil)
	_, pidC := s.Track(context.Background(), "foo", nil)
	require.NotNil(t, pidC)

	// a single PID write is expected and shouldn't block
	pidC <- 111

	gotPidC, tracked := s.PIDReadChan("bar")
	require.False(t, tracked)

	gotPidC, tracked = s.PIDReadChan("foo")
	require.True(t, tracked)
	require.NotNil(t, gotPidC)
	assert.Equal(t, 111, <-gotPidC)
}
