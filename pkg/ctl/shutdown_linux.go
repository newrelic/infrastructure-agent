// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ctl

import (
	ctx "context"
	"fmt"
	"os"
	"sync"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/pkg/errors"
)

const (
	powerOff                   = "poweroff.target"
	jobStatusStart             = "start"
	systemBusAddressFormat     = "unix:path=%s"
	systemBusDefaultPath       = "/run/dbus/system_bus_socket"
	dbusSystemBusAddressEnvVar = "DBUS_SYSTEM_BUS_ADDRESS"
)

var errNoSystemd = errors.New("no systemd found")

func newMonitor() shutdownWatcher {
	// shutdownWatcherLinux uses a separate context to main one in agent as we want to guarantee
	// we read from the dbus even when the agent is shutting down. If we use the agent's context
	// the monitor might return before reading a shutdown event of the channel
	ctx2, cancel := ctx.WithCancel(ctx.Background())
	return &shutdownWatcherLinux{
		connectFunc: connectToDbus,
		ctx:         ctx2,
		cancel:      cancel,
	}
}

type shutdownWatcherLinux struct {
	conn        dbusConn
	connectFunc func() (conn dbusConn, err error)
	ctx         ctx.Context
	cancel      ctx.CancelFunc
	l           sync.RWMutex
}

type dbusConn interface {
	Close()
	ListJobs() ([]dbus.JobStatus, error)
}

func (s *shutdownWatcherLinux) init() (err error) {
	s.l.Lock()
	defer s.l.Unlock()

	nlog.Debug("Init shutdownWatcherLinux.")
	conn, err := s.connectFunc()
	if err != nil {
		nlog.Warn("failed to connect to DBus. make sure systemd is present.")
		return errNoSystemd
	}
	s.conn = conn
	return err
}

func (s *shutdownWatcherLinux) stop() {
	s.l.Lock()
	defer s.l.Unlock()

	s.cancel()
	if s.conn != nil {
		s.conn.Close()
	}
	s.conn = nil
}

func (s *shutdownWatcherLinux) checkShutdownStatus(shutdown chan<- shutdownCmd) {
	s.l.RLock()
	defer s.l.RUnlock()

	// we may have failed to connect to Dbus (no systemd?), so this can be nil
	if s.conn == nil {
		return
	}
	jobs, err := s.conn.ListJobs()
	if err != nil {
		nlog.WithError(err).Warn("error when trying to list jobs")
	} else {
		var found bool
		for _, j := range jobs {
			// did we find poweroff unit starting?
			if j.Unit == powerOff && j.JobType == jobStatusStart {
				nlog.
					WithField("id", j.Id).
					WithField("unit", j.Unit).
					WithField("type", j.JobType).
					WithField("status", j.Status).
					Debug("Found poweroff job.")
				found = true
				break
			}
		}
		if found {
			nlog.Debug("Shutdown in progress detected.")
			shutdown <- shutdownCmd{}
		} else {
			nlog.Debug("No shutdown in progress detected.")
		}
	}
}

func connectToDbus() (conn dbusConn, err error) {
	if _, fnd := os.LookupEnv(dbusSystemBusAddressEnvVar); !fnd {
		_ = os.Setenv(dbusSystemBusAddressEnvVar, getSystemBusPlatformAddress())
	}
	c, e := dbus.New()
	return c, e
}

func getSystemBusPlatformAddress() string {
	hostVar := helpers.HostVar(systemBusDefaultPath)
	return fmt.Sprintf(systemBusAddressFormat, hostVar)
}
