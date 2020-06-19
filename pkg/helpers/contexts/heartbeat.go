// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// package contexts provide extra context implementations
package contexts

import (
	"context"
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
}

// Actuator allows operating with a heartbeatable context
type Actuator struct {
	// Cancel cancels the context
	Cancel context.CancelFunc
	// HeartBeat extends the context life time by the value the context was created with
	HeartBeat func()
}

// WithHeartBeat with return a context that is automatically cancelled if the HeartBeat function
// from the returned Actuator is not invoked periodically before the passed lifetime expires.
func WithHeartBeat(parent context.Context, lifeTime time.Duration) (context.Context, Actuator) {
	ctx := heartBeatCtx{lifeTime: lifeTime}
	actuator := Actuator{HeartBeat: ctx.heartBeat}
	ctx.Context, actuator.Cancel = context.WithCancel(parent)
	ctx.timer = time.AfterFunc(lifeTime, actuator.Cancel)
	return &ctx, actuator
}

func (ctx *heartBeatCtx) heartBeat() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	if !ctx.timer.Stop() {
		<-ctx.timer.C
	}
	ctx.timer.Reset(ctx.lifeTime)
}
