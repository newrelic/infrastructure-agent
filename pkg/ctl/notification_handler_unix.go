// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package ctl

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/os/api/signals"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	shutdownWatcherTimeout = time.Second
)

type shutdownCmd struct {
	noop bool
}

// Interface for basic shutdown watcher
type shutdownWatcher interface {
	// init initialises the shutdown watcher
	init() (err error)

	// stop stops the shutdown watcher.
	// This should cause the monitor function to return
	stop()

	// check for poweroff target
	checkShutdownStatus(shutdown chan<- shutdownCmd)
}

// NotificationHandler executes the handler when a notification is received.
// In Unix notifications are defined as SIGUSR1 signals.
func NotificationHandler(ctx context.Context, handlers map[ipc.Message]func() error) error {
	if handlers == nil || len(handlers) == 0 {
		return errors.New("notification handlers not set")
	}

	shutdownChan := make(chan shutdownCmd, 1) // make it none blocking for noop actions
	sm := newMonitor()
	err := sm.init()
	if err != nil {
		nlog.WithError(err).Warn("failed to init shutdown monitor")
	}

	retCh := make(chan ipc.Message, 1)
	go handleSignals(retCh, shutdownChan, sm)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case msg := <-retCh:
				nlog.WithField("message", msg).Debug("Received notification message.")
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

func processShutdownWatcher(shutdownChan chan shutdownCmd, watcher shutdownWatcher) (found bool) {

	if watcher == nil {
		return false
	}

	watcher.checkShutdownStatus(shutdownChan)

	select {
	case shutdownCmd := <-shutdownChan:
		if !shutdownCmd.noop {
			log.Debug("Got a planned shutdown.")
			found = true
		} else {
			log.Debug("Got a noop shutdown command.")
		}
	case <-time.After(shutdownWatcherTimeout): // wait just to make sure we get an event
		log.Debug("No scheduled shutdown detected.")
	}

	watcher.stop()
	return found
}

func handleSignals(retCh chan<- ipc.Message, shutdownCh chan shutdownCmd, sdw shutdownWatcher) {
	s := make(chan os.Signal, 1)
	signal.Notify(s, signals.Notification, signals.GracefulStop, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sig := <-s:
			nlog.WithField("signal", sig).Info(" Received signal")
			switch sig {
			case signals.GracefulStop, syscall.SIGINT, syscall.SIGTERM:
				if processShutdownWatcher(shutdownCh, sdw) {
					retCh <- ipc.Shutdown
				} else {
					retCh <- ipc.Stop
				}

			case signals.Notification:
				retCh <- ipc.EnableVerboseLogging
			default:
				nlog.WithField("signal", sig).Info("did not recognise received signal")
			}
		}
	}
}
