// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ctl

import (
	"bufio"
	"context"
	"errors"
	"net"

	"github.com/newrelic/infrastructure-agent/pkg/helpers/windows"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
)

// NotificationHandler executes the handler when a notification is received.
// Windows implementation works by having the agent create a NamedPipe.
// The NamedPipe is used to send a (string) message that is picked-up by this handler
func NotificationHandler(ctx context.Context, handlers map[ipc.Message]func() error) error {
	if handlers == nil || len(handlers) == 0 {
		return errors.New("notification handlers not set")
	}

	l, err := windows.NewNotificationPipeListener(windows.GetPipeName("newrelic-infra"))
	if err != nil {
		nlog.WithError(err).Error("failed to create NamedPipe listener")
		return err
	}

	retCh := make(chan ipc.Message, 1)
	// don't block while waiting for connections
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				nlog.WithError(err).Error("failed to accept connection")
				break
			}
			defer conn.Close()

			nlog.Debug("New incoming connection accepted.")
			handleRequest(conn, retCh)
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				l.Close()
				return
			case msg := <-retCh:
				h := handlers[msg]
				if h != nil {
					err := h()
					if err != nil {
						nlog.WithError(err).Error("handler returned error")
					}
				} else {
					nlog.WithField("message", msg).Warn("no handler found for received message. ignoring...")
				}

			}
		}
	}()

	return nil
}

func handleRequest(conn net.Conn, ch chan<- ipc.Message) {
	reader := bufio.NewReader(conn)
	// read everything until newline (including)
	s, err := reader.ReadString('\n')
	if err != nil {
		nlog.WithError(err).Error("Failed to read from pipe")
		return
	}
	// "parse" the message. if it's not a recognized message it will just be ignored
	// the message contains a "\n" so ignore it
	message := ipc.Message(s[:len(s)-1])
	ch <- message
}
