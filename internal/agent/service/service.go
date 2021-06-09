// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
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
	exitCodeC  chan int // wait for the goroutine to exit when stopping the agent on windows.
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
	err = waitForExitOrTimeout(svc.daemon.exitCodeC)
	if err != nil {
		// the agent process did not exit in the allocated time.
		// make sure it doesn't stay around..
		if err == GracefulExitTimeoutErr {
			svc.daemon.cmd.Process.Kill()
		}
		if errCode, ok := err.(*api.ExitCodeErr); ok {
			// discouraged https://github.com/kardianos/service/blob/v1.2.0/service.go#L327
			os.Exit(errCode.ExitCode())
		}
	}
	return err
}

func waitForExitOrTimeout(exitCode <-chan int) error {
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
