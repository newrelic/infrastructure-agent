// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ctl

import (
	"context"
	"errors"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errListenerError = errors.New("listener error")

func TestNewNotificationHandlerWithCancellation(t *testing.T) {
	t.Run("with background context", func(t *testing.T) {
		handler := NewNotificationHandlerWithCancellation(context.Background())
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.ctx)
		assert.NotNil(t, handler.cancel)
		assert.NotNil(t, handler.handlers)
		assert.NotNil(t, handler.listener)
	})

	t.Run("with nil context", func(t *testing.T) {
		handler := NewNotificationHandlerWithCancellation(context.TODO())
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.ctx)
		assert.NotNil(t, handler.cancel)
		assert.NotNil(t, handler.handlers)
		assert.NotNil(t, handler.listener)
	})

	t.Run("with already cancelled context", func(t *testing.T) {
		handler := NewNotificationHandlerWithCancellation(cancelledContext())
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.ctx)
		assert.NotNil(t, handler.cancel)
		assert.NotNil(t, handler.handlers)
		assert.NotNil(t, handler.listener)
	})
}

func TestNotificationHandlerWithCancellation_RegisterHandler(t *testing.T) {
	t.Run("register single handler", func(t *testing.T) {
		handler := NewNotificationHandlerWithCancellation(context.Background())
		handler.RegisterHandler(ipc.Message("TEST"), func() error { return nil })
		assert.Len(t, handler.handlers, 1)
	})

	t.Run("register multiple handlers", func(t *testing.T) {
		handler := NewNotificationHandlerWithCancellation(context.Background())
		handler.RegisterHandler(ipc.Message("TEST2"), func() error { return nil })
		assert.Len(t, handler.handlers, 1)
		handler.RegisterHandler(ipc.Message("ANOTHER"), func() error { return nil })
		assert.Len(t, handler.handlers, 2)
	})
}

func TestNotificationHandlerWithCancellation_Stop(t *testing.T) {
	handler := NewNotificationHandlerWithCancellation(context.Background())

	// Verify context is not cancelled before stop
	select {
	case <-handler.ctx.Done():
		t.Fatal("context should not be cancelled before Stop()")
	default:
	}

	handler.Stop()

	// Verify context is cancelled after stop
	select {
	case <-handler.ctx.Done():
	default:
		t.Fatal("context should be cancelled after Stop()")
	}
}

func TestNotificationHandlerWithCancellation_Start(t *testing.T) {
	t.Run("listener returns no error", func(t *testing.T) {
		handler := NewNotificationHandlerWithCancellation(context.Background())
		handler.listener = func(_ context.Context, _ map[ipc.Message]func() error) error {
			return nil
		}
		err := handler.Start()
		require.NoError(t, err)
	})

	t.Run("listener returns error", func(t *testing.T) {
		handler := NewNotificationHandlerWithCancellation(context.Background())
		handler.listener = func(_ context.Context, _ map[ipc.Message]func() error) error {
			return errListenerError //nolint:wrapcheck
		}
		err := handler.Start()
		require.Error(t, err)
		assert.Equal(t, errListenerError, err)
	})
}

func TestNotificationHandlerWithCancellation_HandlerExecution(t *testing.T) {
	executed := false
	executedMessage := ipc.Message("")

	handler := NewNotificationHandlerWithCancellation(context.Background())

	testHandler := func() error {
		executed = true

		return nil
	}

	handler.RegisterHandler(ipc.Message("TEST_MSG"), testHandler)

	// Replace listener with mock that executes handlers
	handler.listener = func(_ context.Context, handlers map[ipc.Message]func() error) error {
		for msg, h := range handlers {
			executedMessage = msg

			return h()
		}

		return nil
	}

	err := handler.Start()
	require.NoError(t, err)

	assert.True(t, executed)
	assert.Equal(t, ipc.Message("TEST_MSG"), executedMessage)
}

func TestNotificationHandlerWithCancellation_OverwriteHandler(t *testing.T) {
	handler := NewNotificationHandlerWithCancellation(context.Background())

	firstCalled := false
	secondCalled := false

	// Register first handler
	handler.RegisterHandler(ipc.Message("MSG"), func() error {
		firstCalled = true

		return nil
	})

	// Register second handler with same message - should overwrite
	handler.RegisterHandler(ipc.Message("MSG"), func() error {
		secondCalled = true

		return nil
	})

	assert.Len(t, handler.handlers, 1)

	// Execute handler through mock listener
	handler.listener = func(_ context.Context, handlers map[ipc.Message]func() error) error {
		if h, ok := handlers[ipc.Message("MSG")]; ok {
			return h()
		}

		return nil
	}

	err := handler.Start()
	require.NoError(t, err)

	assert.False(t, firstCalled)
	assert.True(t, secondCalled)
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}
