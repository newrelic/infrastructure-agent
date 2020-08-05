// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ctl

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_shutdownWatcherLinux_init(t *testing.T) {
	type fields struct {
	}
	type args struct {
		shutdown chan shutdownCmd
	}
	tests := []struct {
		name           string
		sysmtedEnabled bool
		fields         fields
		args           args
		connMock       *dbusConnMock
		initError      error
	}{
		{
			name:           "happy",
			sysmtedEnabled: true,
			connMock:       &dbusConnMock{},
		},
		{
			name:           "sad",
			sysmtedEnabled: true,
			connMock:       &dbusConnMock{},
			initError:      errors.New("some error"),
		},
		{
			name:           "happy no systemd",
			sysmtedEnabled: false,
			connMock:       &dbusConnMock{},
			initError:      errNoSystemd,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", tt.name)
			assert.NoError(t, err)
			defer os.RemoveAll(dir)

			procDir := filepath.Join(dir, "proc")
			err = os.Mkdir(procDir, 0700)
			assert.NoError(t, err)

			s := newMonitor().(*shutdownWatcherLinux)
			assert.NoError(t, os.Setenv("HOST_PROC", procDir))
			s.connectFunc = func() (conn dbusConn, err error) {
				return tt.connMock, tt.initError
			}

			if tt.sysmtedEnabled {
				assert.NoError(t, setupPid(procDir, "1111", "/lib/systemd/systemd:--user:"))
			}

			err = s.init()
			if tt.initError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_shutdownWatcherLinux_getSystemBusPlatformAddress(t *testing.T) {
	assert.Equal(t, "unix:path=/var/run/dbus/system_bus_socket", getSystemBusPlatformAddress())
	assert.NoError(t, os.Setenv("HOST_VAR", "/some/other/var/"))
	assert.Equal(t, "unix:path=/some/other/var/run/dbus/system_bus_socket", getSystemBusPlatformAddress())
	assert.NoError(t, os.Unsetenv("HOST_VAR"))
}

func Test_shutdownWatcherLinux_checkForShutdownStatus(t *testing.T) {

	connMock := &dbusConnMock{}
	s := newMonitor().(*shutdownWatcherLinux)
	s.connectFunc = func() (conn dbusConn, err error) {
		return connMock, nil
	}
	assert.NoError(t, s.init())

	connMock.On("ListJobs").Return([]dbus.JobStatus{
		{
			Id:       1,
			Unit:     powerOff,
			JobType:  "start",
			Status:   "waiting",
			UnitPath: "ignored",
			JobPath:  "ignored",
		},
	}, nil)
	connMock.On("Close").Once()

	shutdown := make(chan shutdownCmd, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.checkShutdownStatus(shutdown)
		// checkShutdownStatus will write to the shutdown channel
		wg.Done()
	}()

	assert.True(t, processShutdownWatcher(shutdown, s))
	wg.Wait()
	connMock.AssertExpectations(t)
}

type dbusConnMock struct {
	mock.Mock
}

func (d *dbusConnMock) ListJobs() ([]dbus.JobStatus, error) {
	args := d.Called()
	return args.Get(0).([]dbus.JobStatus), nil
}

func (d *dbusConnMock) Close() {
	d.Called()
}

func setupPid(path string, id string, content string) (err error) {
	cmdDir := filepath.Join(path, id)
	err = os.MkdirAll(cmdDir, 0700)
	if nil != err {
		return err
	}
	return ioutil.WriteFile(filepath.Join(cmdDir, "cmdline"), []byte(content), 0644)
}
