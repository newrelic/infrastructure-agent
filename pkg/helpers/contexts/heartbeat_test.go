// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package contexts

import (
	"context"
	"github.com/sirupsen/logrus"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContexHolder_Timeout(t *testing.T) {
	const timeout = 50 * time.Millisecond
	start := time.Now()
	lg := func() *logrus.Entry {
		return logrus.NewEntry(logrus.New())
	}

	// GIVEN a Context with a Heartbeat timeout
	ctx, _ := WithHeartBeat(context.Background(), timeout, lg)

	// WHEN we wait for the heartbeatable context to finish
	select {
	case <-ctx.Done():
	case <-time.After(4 * timeout):
		require.Fail(t, "error waiting for context to be done")
	}
	duration := time.Now().Sub(start)

	// THEN the context finishes with a Canceled error
	assert.Equal(t, context.Canceled, ctx.Err())

	// AND the context does not finishes before the timeout
	assert.Truef(t, duration >= timeout,
		"expected cancellation time %s to be >= than timeout %s", duration, timeout)
}

func TestContextHolder_Heartbeat(t *testing.T) {
	const timeout = 100 * time.Millisecond
	const extendUntil = 200 * time.Millisecond
	start := time.Now()
	lg := func() *logrus.Entry {
		return logrus.NewEntry(logrus.New())
	}

	// GIVEN a Context with a Heartbeat timeout
	ctx, actuator := WithHeartBeat(context.Background(), timeout, lg)

	stopHeartbeating := time.After(extendUntil)

	// WHEN he heartbeat it periodically invoked
hbLoop:
	for {
		select {
		case <-stopHeartbeating:
			break hbLoop
		case <-ctx.Done():
			require.Fail(t, "context should haven't been finished yet")
			break hbLoop
		default:
			time.Sleep(timeout / 10)
			actuator.HeartBeat()
			require.NoError(t, ctx.Err())
		}
	}

	// THEN it does not finish until we stop heartbeating
	// (and wait for the timeout to expire)
	select {
	case <-ctx.Done():
	case <-time.After(4 * timeout):
		require.Fail(t, "error waiting for context to be done")
	}
	duration := time.Now().Sub(start)

	// THEN the context finishes with a Canceled error
	assert.Equal(t, context.Canceled, ctx.Err())

	// AND the context does not finishes before the timeout plus all the extensions
	assert.Truef(t, duration >= extendUntil,
		"expected cancellation time %s to be >= than timeout %s", duration, timeout)
}

func TestContextHolder_Cancel(t *testing.T) {
	// GIVEN a context with a heartbeat timeout
	lg := func() *logrus.Entry {
		return logrus.NewEntry(logrus.New())
	}

	ctx, actuator := WithHeartBeat(context.Background(), 30*time.Second, lg)

	// WHEN we stop the heartbeat, the context must be canceled
	actuator.HeartBeatStop()

	// THEN the context is done before it expired
	select {
	case <-ctx.Done():
	default:
		require.Fail(t, "Context should have been cancelled")
		return
	}

	// AND the context returns no error
	require.Error(t, context.Canceled, ctx.Err())
}

// execute with -race flag
func TestContextHolder_RaceConditions(t *testing.T) {
	const timeout = 50 * time.Millisecond
	const extendUntil = 200 * time.Millisecond
	lg := func() *logrus.Entry {
		return logrus.NewEntry(logrus.New())
	}

	ctx, actuator := WithHeartBeat(context.Background(), timeout, lg)
	waitForAllHeartbeats := time.After(2 * extendUntil)

	for i := 0; i < 100; i++ {
		go func() {
			stopHeartbeating := time.After(extendUntil)
			for {
				select {
				case <-stopHeartbeating:
					return
				case <-ctx.Done():
					require.Fail(t, "context should haven't been finished yet")
					return
				default:
					time.Sleep(timeout / 10)
					actuator.HeartBeat()
				}
			}
		}()
	}

	<-waitForAllHeartbeats

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		require.Fail(t, "error waiting for context to be done")
	}
}
