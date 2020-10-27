package v4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_stoppablesTracker(t *testing.T) {
	s := newStoppablesTracker()
	ctx := s.Add(context.Background(), "foo")

	select {
	case <-ctx.Done():
		t.Fail()
	default:
	}

	stopped := s.Stop("foo")

	select {
	case <-ctx.Done():
	default:
		t.Fail()
	}

	assert.True(t, stopped)

	// once stopped context is removed from track

	_, ok := s.hash2Cancel["foo"]
	assert.False(t, ok)
}

func Test_stoppablesTracker_WontStopNonTrackedContext(t *testing.T) {
	s := newStoppablesTracker()
	ctx := s.Add(context.Background(), "foo")

	select {
	case <-ctx.Done():
		t.Fail()
	default:
	}

	stopped := s.Stop("bar")

	select {
	case <-ctx.Done():
		t.Fail()
	default:
	}

	assert.False(t, stopped)
}
