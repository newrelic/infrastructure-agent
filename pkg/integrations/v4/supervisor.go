// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	ctx2 "context"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

//nolint:gochecknoglobals
var maxBackOff = 5 * time.Minute

// cmdExitStatus is used to signal the outcome of the last process execution.
type cmdExitStatus int

const (
	componentName = "integrations.Supervisor"
)

const (
	statusRunning cmdExitStatus = iota - 1
	statusSuccess
	statusError
)

type Executor interface {
	// When writable a PID channel is provided, generated PID will be written.
	Execute(ctx context.Context, pidChan, exitCodeCh chan<- int) executor.OutputReceive
}

type ParseProcessOutput func(line string) (sanitizedLine string, severity logrus.Level)

// Supervisor is a wrapper for starting and supervising external processes.
type Supervisor struct {
	listenAgentIDChanges   id.UpdateNotifyFn
	hostnameChangeNotifier hostname.ChangeNotifier
	listenRestartRequests  func(ctx ctx2.Context, signalRestart chan<- struct{})

	getBackOffTimer func(time.Duration) *time.Timer
	handleErrs      func(ctx ctx2.Context, errs <-chan error) (hadErrors bool)
	buildExecutor   func() (Executor, error)
	log             log.Entry
	parseOutputFn   ParseProcessOutput
	traceOutput     bool

	preRunActions  func(ctx ctx2.Context)
	postRunActions func(ctx ctx2.Context, exitStatus cmdExitStatus)

	restartCh chan struct{}
}

func (s *Supervisor) Restart() error {
	s.restartCh <- struct{}{}

	return nil
}

func (s *Supervisor) Run(ctx ctx2.Context) {
	s.listenRestartRequests(ctx, s.restartCh)

	// Listen for entity ID updates.
	s.listenAgentIDChanges(s.restartCh, id.NotifyOnReconnect)

	hostnameUpdateCh := make(chan hostname.ChangeNotification, 1)
	s.registerHostnameObserver(hostnameUpdateCh)
	defer s.unRegisterHostnameObserver()

	retryBO := backoff.NewDefaultBackoff()

	for {
		executor, err := s.buildExecutor()
		if err != nil {
			s.log.WithError(err).Error("cannot build supervisor executor")
			select {
			case <-s.restartCh:
				continue
			case <-ctx.Done():
				return
			}
		}

		startTime := time.Now()
		cancel, exitStatus := s.startBackgroundProcess(ctx, executor)

		select {
		case <-s.restartCh:
			cancel()
			<-exitStatus // Wait for the process to exit.
		case change := <-hostnameUpdateCh:
			// make sure to only restart if the hostname change includes the short hostname
			if change.What == hostname.Short || change.What == hostname.ShortAndFull {
				cancel()
				<-exitStatus
			}
		case status := <-exitStatus:
			select {
			case <-ctx.Done():
				return
			default:
			}
			if status == statusSuccess ||
				time.Since(startTime) > maxBackOff {
				retryBO.Reset()
				continue
			}

			retryBOAfter := retryBO.DurationWithMax(maxBackOff)
			s.log.WithField("backOff duration", retryBOAfter).Debug("Supervisor backOff.")

			s.backOff(ctx, retryBOAfter)
		}
	}
}

func (s *Supervisor) startBackgroundProcess(ctx ctx2.Context, executor Executor) (cancel ctx2.CancelFunc, exitStatus chan cmdExitStatus) {
	exitStatus = make(chan cmdExitStatus, 1)

	ctx, cancel = ctx2.WithCancel(ctx)
	go func() {
		if s.preRunActions != nil {
			s.preRunActions(ctx)
		}
		status := s.startProcess(ctx, executor)
		if s.postRunActions != nil {
			s.postRunActions(ctx, status)
		}
		exitStatus <- status
	}()
	return
}

func (s *Supervisor) startProcess(ctx ctx2.Context, executor Executor) cmdExitStatus {
	s.log.Debug("Launching process.")
	cmdOutputPipe := executor.Execute(ctx, nil, nil)

	go s.handleStdOut(cmdOutputPipe.Stdout)
	go s.handleStdErr(cmdOutputPipe.Stderr)

	hadError := make(chan bool)
	go func() {
		hadError <- s.handleErrs(ctx, cmdOutputPipe.Errors)
	}()
	if <-hadError {
		return statusError
	}
	return statusSuccess
}

// backOff waits for the specified duration or a signal from the stop
// channel, whichever happens first.
func (s *Supervisor) backOff(ctx ctx2.Context, d time.Duration) {
	backOffTimer := s.getBackOffTimer(d)
	select {
	case <-backOffTimer.C:
	case <-ctx.Done():
		return
	}
}

func (s *Supervisor) handleStdOut(stdout <-chan []byte) {
	for out := range stdout {
		s.logLine(out, "stdout")
	}
}

func (s *Supervisor) handleStdErr(stderr <-chan []byte) {
	for err := range stderr {
		s.logLine(err, "stderr")
	}
}

func (s *Supervisor) logLine(out []byte, channel string) {
	strOut := string(out)
	var saneLine string
	var lvl logrus.Level
	// avoid feedback loops
	if !strings.Contains(strOut, componentName) {
		if s.traceOutput {
			tLog := s.log.WithField(config.TracesFieldName, config.SupervisorTrace)
			tLog.Trace(strOut)
			return
		}

		saneLine, lvl = s.parseOutputFn(strOut)
		if saneLine == "" {
			return
		}

		l := s.log.WithField("output", channel)
		// FB debug & info stuff should log in agent as debug content
		if lvl >= logrus.InfoLevel {
			l.Debug(saneLine)
		} else if lvl >= logrus.WarnLevel {
			l.Warn(saneLine)
		} else if lvl >= logrus.ErrorLevel {
			l.Error(saneLine)
		}
	}
}

func handleErrors(log log.Entry) func(ctx ctx2.Context, errs <-chan error) (hadErrors bool) {
	return func(ctx ctx2.Context, errs <-chan error) (hadErrors bool) {
		for {
			select {
			case <-ctx.Done():
				// don't log errors if the context has been just cancelled
				return
			case err, open := <-errs:
				if !open {
					return
				}
				hadErrors = true
				log.WithError(err).Error("Error occurred while handling the process")
			}
		}
	}
}

func (s *Supervisor) registerHostnameObserver(ch chan<- hostname.ChangeNotification) {
	s.hostnameChangeNotifier.AddObserver(ObserverName, ch)
}

func (s *Supervisor) unRegisterHostnameObserver() {
	s.hostnameChangeNotifier.RemoveObserver(ObserverName)
}
