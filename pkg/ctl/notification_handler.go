// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ctl

import (
	"context"
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var nlog = log.WithComponent("NotificationHandler")

// NotificationHandlerWithCancellation wraps NotificationHandler with a cancellation context.
type NotificationHandlerWithCancellation struct {
	ctx      context.Context
	cancel   context.CancelFunc
	handlers map[ipc.Message]func() error
	listener func(ctx context.Context, handlers map[ipc.Message]func() error) error
}

// NewNotificationHandlerWithCancellation creates a new instance of NotificationHandlerWithCancellation.
func NewNotificationHandlerWithCancellation(ctx context.Context) *NotificationHandlerWithCancellation {
	if ctx == nil {
		ctx = context.Background()
	}

	//c := make(chan NotificationMessage, 1)

	ctx, cancel := context.WithCancel(ctx)
	return &NotificationHandlerWithCancellation{
		ctx:      ctx,
		cancel:   cancel,
		handlers: make(map[ipc.Message]func() error),
		listener: NotificationHandler,
	}
}

// RegisterHandler register a message handler
func (n *NotificationHandlerWithCancellation) RegisterHandler(message ipc.Message, handler func() error) {
	nlog.Debug(fmt.Sprintf("registering message handler: %s", message))
	n.handlers[message] = handler
}

// Start starts the handler in a separate go routine.
func (n *NotificationHandlerWithCancellation) Start() error {
	return n.listener(n.ctx, n.handlers)
}

// Stop the handler.
func (n *NotificationHandlerWithCancellation) Stop() {
	n.cancel()
}
