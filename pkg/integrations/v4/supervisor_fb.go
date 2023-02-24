// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	ctx2 "context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	"github.com/pkg/errors"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var sFBLogger = log.WithComponent("integrations.Supervisor").WithField("process", "log-forwarder")
var luaFilterTempFileRegex = regexp.MustCompile("nr_fb_lua_filter\\d+")

type FBSupervisorConfig struct {
	FluentBitExePath     string
	FluentBitNRLibPath   string
	FluentBitParsersPath string
	FluentBitVerbose     bool
}

const (
	MaxNumberOfFbConfigTempFiles int = 50
)

// listError error representing a list of errors.
type listError struct {
	Errors []error
}

func (s *listError) Error() (err string) {
	err = "List of errors:"
	for _, e := range s.Errors {
		err += fmt.Sprintf(" - %s", e.Error())
	}

	return err
}

func (s *listError) Add(e error) { s.Errors = append(s.Errors, e) }

// ErrorOrNil returns an error interface if the Error slice is not empty or nil otherwise.
func (s *listError) ErrorOrNil() error {
	if s == nil || len(s.Errors) == 0 {
		return nil
	}

	return s
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

var ObserverName = "LogForwarderSupervisor" // nolint:gochecknoglobals

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
		traceOutput:            fbIntCfg.FluentBitVerbose,
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

		removedFbConfigTempFiles, err := removeFbConfigTempFiles(MaxNumberOfFbConfigTempFiles)
		if err != nil {
			log.WithError(err).Warn("Failed removing config temp files.")
		}

		for _, file := range removedFbConfigTempFiles {
			log.Debugf("Removed %s config temp file.", file)
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

		if fbIntCfg.FluentBitVerbose {
			args = append(args, "-vv")
		}

		fbExecutor := executor.FromCmdSlice(args, &executor.Config{
			IntegrationName: "fluent-bit",
			Environment: map[string]string{
				"NR_LICENSE_KEY_ENV_VAR": cfgLoader.GetLicenseKey(),
			},
		})
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

// keeps the most recent config files, up to maxNumberOfFbConfigTempFiles, and removes the rest.
func removeFbConfigTempFiles(maxNumberOfFbConfigTempFiles int) ([]string, error) {
	fbConfigTempFiles, err := readFbConfigTempFilesFromTempDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed reading config temp files: %w", err)
	}

	if len(fbConfigTempFiles) <= maxNumberOfFbConfigTempFiles {
		return nil, nil
	}

	// sort fbConfigTempFiles by ascending modification date
	sort.Slice(fbConfigTempFiles, func(i, j int) bool {
		fileInfo1, _ := fbConfigTempFiles[i].Info()
		fileInfo2, _ := fbConfigTempFiles[j].Info()

		return fileInfo1.ModTime().Before(fileInfo2.ModTime())
	})

	var removedConfigTempFiles []string
	var configTempFilesToRemove []string
	var listErrors listError

	// create list of fbConfigTempFiles to remove
	for i := 0; i < len(fbConfigTempFiles)-maxNumberOfFbConfigTempFiles; i++ {
		configTempFilesToRemove = append(configTempFilesToRemove, fbConfigTempFiles[i].Name())
	}

	// extract lua filter filenames from config temp files to remove
	for _, configTempFileToRemove := range configTempFilesToRemove {
		if fbLuaFilterTempFilenames, err := extractLuaFilterFilenames(configTempFileToRemove); err != nil {
			listErrors.Add(err)
		} else {
			configTempFilesToRemove = append(configTempFilesToRemove, fbLuaFilterTempFilenames...)
		}
	}

	// remove all config and lua filter temp files from temporary directory
	for _, configTempFileToRemove := range configTempFilesToRemove {
		if err := os.Remove(filepath.Join(os.TempDir(), configTempFileToRemove)); err != nil {
			listErrors.Add(err)
		} else {
			removedConfigTempFiles = append(removedConfigTempFiles, configTempFileToRemove)
		}
	}

	return removedConfigTempFiles, listErrors.ErrorOrNil()
}

// return the list of temp config files from temporary directory.
func readFbConfigTempFilesFromTempDirectory() ([]fs.DirEntry, error) {
	files, err := os.ReadDir(os.TempDir())
	if err != nil {
		return nil, fmt.Errorf("failed reading temp directory: %w", err)
	}

	var fbConfigTempFiles []fs.DirEntry

	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "nr_fb_config") {
			fbConfigTempFiles = append(fbConfigTempFiles, file)
		}
	}

	return fbConfigTempFiles, nil
}

// extract lua filter temp filenames referenced by fbConfigTempFilename.
func extractLuaFilterFilenames(fbConfigTempFilename string) ([]string, error) {
	fbConfigTempFileContent, err := os.ReadFile(filepath.Join(os.TempDir(), fbConfigTempFilename))
	if err != nil {
		return nil, fmt.Errorf("failed reading config temp file: %s error: %w", fbConfigTempFilename, err)
	}

	luaFilterTempFilenames := luaFilterTempFileRegex.FindAllString(string(fbConfigTempFileContent), -1)

	return luaFilterTempFilenames, nil
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
