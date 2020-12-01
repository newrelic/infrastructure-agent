package stoppable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoppablesTracker_Add(t *testing.T) {
	s := NewTracker()
	ctx, _ := s.Track(context.Background(), "foo")

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

	_, ok := s.hash2Cancel["foo"]
	assert.False(t, ok, "once stopped context should had been removed from track")
}

func TestStoppablesTracker_Kill_WontStopNonTrackedContext(t *testing.T) {
	s := NewTracker()
	ctx, _ := s.Track(context.Background(), "foo")

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

func TestStoppablesTracker_PID(t *testing.T) {
	s := NewTracker()
	_, pidC := s.Track(context.Background(), "foo")
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
