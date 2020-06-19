// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	ctx2 "context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var sFBLogger = log.WithComponent("integrations.Supervisor").WithField("process", "log-forwarder")

type FBSupervisorConfig struct {
	FluentBitExePath     string
	FluentBitNRLibPath   string
	FluentBitParsersPath string
}

// IsLogForwarderAvailable checks whether all the required files for FluentBit execution are available
func (c *FBSupervisorConfig) IsLogForwarderAvailable() bool {

	if _, err := os.Stat(c.FluentBitExePath); err != nil {
		sFBLogger.WithField("fbExePath", c.FluentBitExePath).Debug("Fluent Bit exe not found.")
		return false
	}
	if _, err := os.Stat(c.FluentBitNRLibPath); err != nil {
		sFBLogger.WithField("fbNROutLibPath", c.FluentBitNRLibPath).Debug("Fluent Bit NR output plugin not found.")
		return false
	}
	if _, err := os.Stat(c.FluentBitParsersPath); err != nil {
		sFBLogger.WithField("fbParsersPath", c.FluentBitNRLibPath).Debug("Default parsers.conf not found.")
		return false
	}

	return true
}

// SendEventFn wrapper for sending events to nr.
type SendEventFn func(event sample.Event, entityKey entity.Key)

var (
	ObserverName = "LogForwarderSupervisor"
)

// NewFBSupervisor builds a Fluent Bit supervisor which forwards the output to agent logs.
func NewFBSupervisor(fbIntCfg FBSupervisorConfig, cfgLoader *logs.CfgLoader, agentIDNotifier id.UpdateNotifyFn, notifier hostname.ChangeNotifier, sendEventFn SendEventFn) *Supervisor {
	return &Supervisor{
		listenAgentIDChanges:   agentIDNotifier,
		hostnameChangeNotifier: notifier,
		listenRestartRequests:  listenRestartRequests(cfgLoader),
		getBackOffTimer:        time.NewTimer,
		handleErrs:             handleErrors(sFBLogger),
		buildExecutor:          buildFbExecutor(fbIntCfg, cfgLoader),
		log:                    sFBLogger,
		preRunActions:          fbPreRunActions(sendEventFn),
		postRunActions:         fbPostRunActions(sendEventFn),
		parseOutputFn:          logs.ParseFBOutput,
	}
}

func fbPreRunActions(sendEventFn SendEventFn) func(ctx2.Context) {
	return func(ctx2.Context) {
		event := NewSupervisorEvent("Fluent Bit Started", statusRunning)
		sendEventFn(event, entity.EmptyKey)
	}
}

func fbPostRunActions(sendEventFn SendEventFn) func(ctx2.Context, cmdExitStatus) {
	return func(ctx ctx2.Context, exitCode cmdExitStatus) {
		event := NewSupervisorEvent("Fluent Bit Stopped", exitCode)
		sendEventFn(event, entity.EmptyKey)
	}
}

// buildFbExecutor builds the function required by supervisor when running the process.
func buildFbExecutor(fbIntCfg FBSupervisorConfig, cfgLoader *logs.CfgLoader) func() (Executor, error) {
	return func() (Executor, error) {

		cfgContent, externalCfg, cErr := cfgLoader.LoadAndFormat()
		if cErr != nil {
			return nil, cErr
		}

		cfgTmpPath, err := saveToTempFile([]byte(cfgContent))
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temporary fb sFBLogger config file")
		}

		args := []string{
			fbIntCfg.FluentBitExePath,
			"-c",
			cfgTmpPath,
			"-e",
			fbIntCfg.FluentBitNRLibPath,
			"-R",
			fbIntCfg.FluentBitParsersPath,
		}

		if (externalCfg != logs.FBCfgExternal{} && externalCfg.ParsersFilePath != "") {
			args = append(args, "-R", externalCfg.ParsersFilePath)
		}

		fbExecutor := executor.FromCmdSlice(args, &executor.Config{})
		return &fbExecutor, nil
	}
}

// returns the file name
func saveToTempFile(config []byte) (string, error) {
	// create it
	file, err := ioutil.TempFile("", "nr_fb_config")
	if err != nil {
		return "", err
	}
	defer file.Close()

	sFBLogger.WithField("file", file.Name()).WithField("content", string(config)).
		Debug("Creating temp config file for fb sFBLogger.")

	if _, err := file.Write(config); err != nil {
		return "", err
	}
	return file.Name(), nil
}

// SupervisorEvent will be used to create an InfrastructureEvent when fb start/stop.
type SupervisorEvent struct {
	sample.BaseEvent
	Summary    string `json:"summary"`
	ExitStatus string `json:"exitStatus"`
}

// NewSupervisorEvent create a new SupervisorEvent instance. For running ongoing pass -1.
func NewSupervisorEvent(eventSummary string, status cmdExitStatus) *SupervisorEvent {
	return &SupervisorEvent{
		BaseEvent: sample.BaseEvent{
			EventType: "InfrastructureEvent",
			Timestmp:  time.Now().Unix(),
		},
		Summary:    eventSummary,
		ExitStatus: fmt.Sprintf("%d", status),
	}
}

func listenRestartRequests(cfgLoader *logs.CfgLoader) func(ctx ctx2.Context, signalRestart chan<- struct{}) {
	cw := logs.NewConfigChangesWatcher(cfgLoader.GetConfigDir())
	return func(ctx ctx2.Context, signalRestart chan<- struct{}) {
		cw.Watch(ctx, signalRestart)
	}
}
