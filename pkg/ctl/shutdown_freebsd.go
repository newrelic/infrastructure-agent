// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ctl

func newMonitor() shutdownWatcher {
	return &shutdownWatcherDarwin{}
}

type shutdownWatcherDarwin struct {
}

func (s *shutdownWatcherDarwin) checkShutdownStatus(shutdown chan<- shutdownCmd) {
	shutdown <- shutdownCmd{noop: true}
}

func (s *shutdownWatcherDarwin) init() (err error) {
	return err
}

func (s *shutdownWatcherDarwin) stop() {}
