// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	ctx2 "context"
	"fmt"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/stretchr/testify/assert"
)

func TestSupervisor_RestartOnEntityIDChange(t *testing.T) {
	notifierMock := NewNotifierMock()
	supervisorMock := NewSupervisorMock(notifierMock)

	ctx, cancel := ctx2.WithCancel(ctx2.Background())
	defer cancel()

	go supervisorMock.Start(ctx)

	for _, expectedTestCalls := range []string{
		"supervisor_addobserver",
		"build_executor",
		"handle_errs",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)

	supervisorMock.updateEntityID()
	for _, expectedTestCalls := range []string{
		"update_entity_id",
		"executor_done",
		"build_executor",
		"handle_errs",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)
}

func TestSupervisor_SupervisorStop(t *testing.T) {
	notifierMock := NewNotifierMock()
	supervisorMock := NewSupervisorMock(notifierMock)

	ctx, cancel := ctx2.WithCancel(ctx2.Background())
	go supervisorMock.Start(ctx)

	for _, expectedTestCalls := range []string{
		"supervisor_addobserver",
		"build_executor",
		"handle_errs",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)

	cancel()
	for _, expectedTestCalls := range []string{
		"executor_done",
		"supervisor_done",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)
}

func TestSupervisor_SupervisorRestartProcessWithBackOff(t *testing.T) {
	t.Skip("Skipping as test seems to fail due to timing issues.")
	notifierMock := NewNotifierMock()
	supervisorMock := NewSupervisorMock(notifierMock)

	ctx, cancel := ctx2.WithCancel(ctx2.Background())
	defer cancel()

	go supervisorMock.Start(ctx)

	for _, expectedTestCalls := range []string{
		"supervisor_addobserver",
		"build_executor",
		"handle_errs",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)

	supervisorMock.executor.triggerFakeError()
	for _, expectedTestCalls := range []string{
		"executor_done",
		"with_backoff",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)
}

func assertTestCalls(t *testing.T, supervisorMock *SupervisorMock, expectedTestCalls string) {
	select {
	case receivedTestCall := <-supervisorMock.testCalls:
		assert.Equal(t, expectedTestCalls, receivedTestCall)
	case <-time.After(2 * time.Second):
		assert.Fail(t, "Timeout exceeded while waiting for test calls stack")
	}
}

func assertNoTestCalls(t *testing.T, supervisorMock *SupervisorMock) {
	select {
	case testCall := <-supervisorMock.testCalls:
		assert.Fail(t, "Received unexpected test call:"+testCall)
	default:
	}
}

// Mocks

type ExecutorMock struct {
	errorCh chan error
}

func NewExecutorMock() *ExecutorMock {
	return &ExecutorMock{
		errorCh: make(chan error, 1),
	}
}

func (em *ExecutorMock) Execute(ctx ctx2.Context, pidChan, exitCodeCh chan<- int) executor.OutputReceive {
	return executor.OutputReceive{
		Errors: em.errorCh,
	}
}

func (em *ExecutorMock) triggerFakeError() {
	em.errorCh <- fmt.Errorf("test error")
}

type SupervisorMock struct {
	entityIDUpdate chan<- struct{}
	executor       *ExecutorMock

	testCalls        chan string
	hostnameNotifier hostname.ChangeNotifier
	hostnameUpdateCh chan<- hostname.ChangeNotification
}

func NewSupervisorMock(mock hostname.ChangeNotifier) *SupervisorMock {
	return &SupervisorMock{
		executor:         NewExecutorMock(),
		testCalls:        make(chan string, 100),
		hostnameNotifier: mock,
		hostnameUpdateCh: make(chan hostname.ChangeNotification),
	}
}

func NewNotifierMock() *NotifierMock {
	return &NotifierMock{}
}

type NotifierMock struct {
	ch chan<- hostname.ChangeNotification
}

func (n *NotifierMock) AddObserver(_ string, ch chan<- hostname.ChangeNotification) {
	n.ch = ch
}

func (n *NotifierMock) RemoveObserver(_ string) {
	n.ch = nil
}

func (n *NotifierMock) UpdateHostname(testCalls chan<- string) {
	testCalls <- "update_hostname"
	n.ch <- hostname.ChangeNotification{}
}

func (sm *SupervisorMock) Start(ctx ctx2.Context) {
	testSupervisor := NewSupervisorFromMock(sm)

	sm.hostnameNotifier.AddObserver("test", sm.hostnameUpdateCh)
	sm.testCalls <- "supervisor_addobserver"

	testSupervisor.Run(ctx)
	sm.testCalls <- "supervisor_done"
}

func (sm *SupervisorMock) buildExecutor() (Executor, error) {
	sm.testCalls <- "build_executor"
	return sm.executor, nil
}

func NewSupervisorFromMock(supervisorMock *SupervisorMock) *Supervisor {
	return &Supervisor{
		buildExecutor:          supervisorMock.buildExecutor,
		handleErrs:             supervisorMock.handleErrs,
		listenAgentIDChanges:   supervisorMock.entityIDNotify,
		listenRestartRequests:  supervisorMock.handleRestart,
		getBackOffTimer:        supervisorMock.getBackOffTimer,
		parseOutputFn:          logs.ParseFBOutput,
		hostnameChangeNotifier: supervisorMock.hostnameNotifier,
		restartCh:              make(chan struct{}, 1),
	}
}

func (sm *SupervisorMock) handleRestart(ctx ctx2.Context, receiver chan<- struct{}) {
}

func (sm *SupervisorMock) entityIDNotify(c chan<- struct{}, _ id.AgentIDNotifyMode) {
	sm.entityIDUpdate = c
}

func (sm *SupervisorMock) updateEntityID() {
	sm.testCalls <- "update_entity_id"
	sm.entityIDUpdate <- struct{}{}
}

func (sm *SupervisorMock) handleErrs(ctx ctx2.Context, errs <-chan error) (hadErrors bool) {
	defer func() {
		sm.testCalls <- "executor_done"
	}()

	sm.testCalls <- "handle_errs"
	select {
	case <-ctx.Done():
		return
	case <-errs:
		hadErrors = true
	}
	return hadErrors
}

func (sm *SupervisorMock) getBackOffTimer(duration time.Duration) *time.Timer {
	sm.testCalls <- "with_backoff"
	return time.NewTimer(0)
}

func TestSupervisor_RestartOnHostnameChange(t *testing.T) {
	notifierMock := NewNotifierMock()
	supervisorMock := NewSupervisorMock(notifierMock)

	ctx, cancel := ctx2.WithCancel(ctx2.Background())
	defer cancel()

	go supervisorMock.Start(ctx)

	for _, expectedTestCalls := range []string{
		"supervisor_addobserver",
		"build_executor",
		"handle_errs",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)

	notifierMock.UpdateHostname(supervisorMock.testCalls)

	for _, expectedTestCalls := range []string{
		"update_hostname",
		"executor_done",
		"build_executor",
		"handle_errs",
	} {
		assertTestCalls(t, supervisorMock, expectedTestCalls)
	}
	assertNoTestCalls(t, supervisorMock)
}
