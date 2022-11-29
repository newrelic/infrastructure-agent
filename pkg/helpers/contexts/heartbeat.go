// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// package contexts provide extra context implementations
package contexts

import (
	"context"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"sync"
	"time"
)

// heartBeatCtx implements a context.Context that is automatically cancelled unless
// periodic heartbeats are triggered
type heartBeatCtx struct {
	context.Context
	timer    *time.Timer
	mutex    sync.Mutex
	lifeTime time.Duration
	// cancel cancels the context
	cancel context.CancelFunc
}

// Actuator allows operating with a heartbeatable context
type Actuator struct {
	// HeartBeat extends the context life time by the value the context was created with
	HeartBeat     func()
	HeartBeatStop func()
}

// WithHeartBeat with return a context that is automatically cancelled if the HeartBeat function
// from the returned Actuator is not invoked periodically before the passed timeout expires.
func WithHeartBeat(parent context.Context, timeout time.Duration, lg log.Entry) (context.Context, Actuator) {
	ctx := heartBeatCtx{
		lifeTime: timeout,
	}

	actuator := Actuator{
		HeartBeat:     ctx.heartBeat,
		HeartBeatStop: ctx.heartBeatStop,
	}
	ctx.Context, ctx.cancel = context.WithCancel(parent)
	ctx.timer = time.AfterFunc(timeout, func() {
		lg.Warnf("HeartBeat timeout exceeded after %f seconds", timeout.Seconds())
		ctx.cancel()
	})
	return &ctx, actuator
}

func (ctx *heartBeatCtx) heartBeat() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// timer.Stop() prevents firing AfterFunc execution.
	// It returns false when already stopped or AfterFunc execution started.
	if !ctx.timer.Stop() {
		// HeartBeat received while the context timeout exceeded or manually stopped.
		<-ctx.Done()
		return
	}

	// HeartBeat received in time, reset the timeout timer.
	ctx.timer.Reset(ctx.lifeTime)
}

func (ctx *heartBeatCtx) heartBeatStop() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	defer ctx.cancel()
	ctx.timer.Stop()
}
