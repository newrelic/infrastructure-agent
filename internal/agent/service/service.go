// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kardianos/service"
	"github.com/newrelic/infrastructure-agent/internal/os/api"
)

const (
	svcName = "newrelic-infra"
)

var (
	GracefulExitTimeout    = 10 * time.Second
	GracefulExitTimeoutErr = errors.New("graceful stop time exceeded... forcing stop")
)

// Service is a wrapper object which provides the necessary hooks to start/stop the agent as a service
type Service struct {
	buildVersion string
	daemon       daemon
}

func New(exitCodeC chan int, arg ...string) (service.Service, error) {
	svc := &Service{
		daemon: daemon{
			args:      arg,
			exitCodeC: exitCodeC,
			exited:    new(atomBool),
		},
	}

	cfg := &service.Config{
		Name: svcName,
	}

	return service.New(svc, cfg)
}

type daemon struct {
	sync.Mutex // daemon can be accessed from different routines.
	args       []string
	cmd        *exec.Cmd
	exitCodeC  chan int // exit status handed off to the service for its own exit
	exited     *atomBool
}

type atomBool struct {
	flag int32
}

func (b *atomBool) Set(value bool) {
	var i int32 = 0
	if value {
		i = 1
	}
	atomic.StoreInt32(&(b.flag), i)
}

func (b *atomBool) Get() bool {
	if atomic.LoadInt32(&(b.flag)) != 0 {
		return true
	}
	return false
}

// GetCommandPath returns the absolute path of the agent binary that should be run.
func GetCommandPath(svcCmd string) string {
	path := filepath.Join(filepath.Dir(svcCmd), svcName)
	if runtime.GOOS == "windows" {
		return path + ".exe"
	}
	return path
}

func (svc *Service) terminate(err error) error {
	err = WaitForExitOrTimeout(svc.daemon.exitCodeC)
	// the agent process did not exit in the allocated time.
	// make sure it doesn't stay around..
	if err == GracefulExitTimeoutErr {
		svc.daemon.cmd.Process.Kill()
	}

	return err
}

func WaitForExitOrTimeout(exitCode <-chan int) error {
	select {
	case <-time.After(GracefulExitTimeout):
		return GracefulExitTimeoutErr
	case c := <-exitCode:
		if c == 0 {
			return nil
		}
		return api.NewExitCodeErr(c)
	}
}
