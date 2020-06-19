// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kardianos/service"
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

func New(arg ...string) (service.Service, error) {
	svc := &Service{
		daemon: daemon{
			args: arg,
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
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup // wait for the goroutine to exit when stopping the agent on windows.
}

// GetCommandPath returns the absolute path of the agent binary that should be run.
func GetCommandPath(svcCmd string) string {
	path := filepath.Join(filepath.Dir(svcCmd), svcName)
	if runtime.GOOS == "windows" {
		return path + ".exe"
	}
	return path
}

func waitForExitOrTimeout(gracefulExit <-chan struct{}) error {
	// wait for run() to finish its execution or timeout
	select {
	case <-time.After(GracefulExitTimeout):
		return GracefulExitTimeoutErr
	case <-gracefulExit:
		return nil
	}
}
