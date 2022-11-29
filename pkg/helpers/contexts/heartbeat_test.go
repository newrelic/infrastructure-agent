// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package contexts

import (
	"context"
	"github.com/sirupsen/logrus"
	"strings"
	"testing"
	"time"

	testlog "github.com/newrelic/infrastructure-agent/test/log"
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
	logger := logrus.New()
	logHook := testlog.NewInMemoryEntriesHook(logrus.AllLevels)
	logger.AddHook(logHook)

	lg := func() *logrus.Entry {
		return logrus.NewEntry(logger)
	}

	// GIVEN a context with a heartbeat timeout
	timeout := 100 * time.Millisecond
	ctx, actuator := WithHeartBeat(context.Background(), timeout, lg)

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

	// AND no HeartBeat warning is logged
	// Wait to exceed the heartbeat timeout before checking the logs
	time.Sleep(timeout * 2)

	assert.Equal(t, 0, countLogEntriesContainingMsg(logHook.GetEntries(), "HeartBeat timeout exceeded after"))
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

func TestContextHolder_LateHeartbeat(t *testing.T) {
	logger := logrus.New()
	logHook := testlog.NewInMemoryEntriesHook(logrus.AllLevels)
	logger.AddHook(logHook)

	start := time.Now()
	lg := func() *logrus.Entry {
		return logrus.NewEntry(logger)
	}

	// GIVEN a Context with a Heartbeat timeout
	const timeout = 100 * time.Millisecond
	ctx, actuator := WithHeartBeat(context.Background(), timeout, lg)

	// WHEN heartbeat is emitted too late
	time.AfterFunc(timeout*2, func() {
		actuator.HeartBeat()
	})

	// THEN the HeartBeatCtx is done with a Canceled error
	var duration time.Duration

	select {
	case <-ctx.Done():
		duration = time.Now().Sub(start)
	case <-time.After(3 * timeout):
		require.Fail(t, "error waiting for context to be done")
	}

	assert.Equal(t, context.Canceled, ctx.Err())

	// AND the context does not finish before the timeout
	assert.Truef(t, duration >= timeout,
		"expected cancellation time %s to be >= than timeout %s", duration, timeout)

	// AND only one HeartBeat warning is logged
	// Wait to exceed another heartbeat timeout after the HeartBeat was submitted
	time.Sleep(timeout * 3)

	assert.Equal(t, 1, countLogEntriesContainingMsg(logHook.GetEntries(), "HeartBeat timeout exceeded after"))
}

func countLogEntriesContainingMsg(entries []logrus.Entry, msg string) int {
	result := 0
	for _, entry := range entries {
		if strings.Contains(entry.Message, msg) {
			result++
		}
	}
	return result
}
