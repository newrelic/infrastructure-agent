// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	ctx2 "context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

const (
	FbConfTempFolderNameDefault      = "fb"
	temporaryFolderPermissions       = 0o755
	MaxNumberOfFbConfigTempFiles int = 50
)

var (
	//nolint:gochecknoglobals
	sFBLogger              = log.WithComponent("integrations.Supervisor").WithField("process", "log-forwarder")
	luaFilterTempFileRegex = regexp.MustCompile(`nr_fb_lua_filter\d+`)
	errFbNotAvailable      = errors.New("cannot build FB executer: FB not available")
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

type fBSupervisorConfig struct {
	agentDir             string
	integrationsDir      string
	loggingBinDir        string
	fluentBitExePath     string
	FluentBitNRLibPath   string
	FluentBitParsersPath string
	FluentBitVerbose     bool
	ConfTemporaryFolder  string
	ffRetriever          feature_flags.Retriever
}

// NewFBSupervisorConfig creates a new fBSupervisorConfig that will contain the FF retriever
// not exported to force using the constructor and ensuring it contains all necessary dependencies
// nolint:revive
func NewFBSupervisorConfig(
	ffRetriever feature_flags.Retriever,
	agentDir string,
	integrationsDir string,
	loggingBinDir string,
	fluentBitExePath string,
	fluentBitNRLibPath string,
	fluentBitParsersPath string,
	fluentBitVerbose bool,
	confTempFolder string,
) fBSupervisorConfig {
	return fBSupervisorConfig{
		ffRetriever:          ffRetriever,
		agentDir:             agentDir,
		integrationsDir:      integrationsDir,
		loggingBinDir:        loggingBinDir,
		fluentBitExePath:     fluentBitExePath,
		FluentBitNRLibPath:   fluentBitNRLibPath,
		FluentBitParsersPath: fluentBitParsersPath,
		FluentBitVerbose:     fluentBitVerbose,
		ConfTemporaryFolder:  confTempFolder,
	}
}

// IsLogForwarderAvailable checks whether all the required files for FluentBit execution are available
func (c *fBSupervisorConfig) IsLogForwarderAvailable() bool {
	fluentBitExePath := c.getFbPath()
	if _, err := os.Stat(fluentBitExePath); err != nil {
		sFBLogger.WithField("fbExePath", fluentBitExePath).Debug("Fluent Bit exe not found.")
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

func (c *fBSupervisorConfig) getFbPath() string {
	// manually set conf always has precedence
	if c.fluentBitExePath != "" {
		return c.fluentBitExePath
	}

	// only if FF exists and it's enabled, be use legacey version 1.9
	enabled, exists := c.ffRetriever.GetFeatureFlag(fflag.FlagFluentBit19)

	// value from config rules
	loggingBinDir := c.loggingBinDir
	if loggingBinDir == "" {
		loggingBinDir = c.defaultLoggingBinDir(exists, enabled)
	}

	return c.defaultFluentBitExePath(exists, enabled, loggingBinDir)
}

// SendEventFn wrapper for sending events to nr.
type SendEventFn func(event sample.Event, entityKey entity.Key)

var ObserverName = "LogForwarderSupervisor" // nolint:gochecknoglobals

// NewFBSupervisor builds a Fluent Bit supervisor which forwards the output to agent logs.
func NewFBSupervisor(fbIntCfg fBSupervisorConfig, cfgLoader *logs.CfgLoader, agentIDNotifier id.UpdateNotifyFn, notifier hostname.ChangeNotifier, sendEventFn SendEventFn) *Supervisor {
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
		restartCh:              make(chan struct{}, 1),
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
func buildFbExecutor(fbIntCfg fBSupervisorConfig, cfgLoader *logs.CfgLoader) func() (Executor, error) {
	return func() (Executor, error) {
		if !fbIntCfg.IsLogForwarderAvailable() {
			return nil, errFbNotAvailable
		}

		cfgContent, externalCfg, cErr := cfgLoader.LoadAndFormat()
		if cErr != nil {
			return nil, cErr
		}

		cfgTmpPath, err := saveToTempFile(fbIntCfg.ConfTemporaryFolder, []byte(cfgContent))
		if err != nil {
			return nil, errors.Wrap(err, "failed to create temporary fb sFBLogger config file")
		}

		removedFbConfigTempFiles, err := removeFbConfigTempFiles(fbIntCfg.ConfTemporaryFolder, MaxNumberOfFbConfigTempFiles)
		if err != nil {
			log.WithError(err).Warn("Failed removing config temp files.")
		}

		for _, file := range removedFbConfigTempFiles {
			log.Debugf("Removed %s config temp file.", file)
		}

		args := []string{
			fbIntCfg.getFbPath(),
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
func saveToTempFile(tempDir string, config []byte) (string, error) {
	// ensure that tempdir exits
	err := os.MkdirAll(tempDir, temporaryFolderPermissions)
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary folder for fluent-bit")
	}

	// create it
	file, err := os.CreateTemp(tempDir, "nr_fb_config")
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
func removeFbConfigTempFiles(tempDir string, maxNumberOfFbConfigTempFiles int) ([]string, error) {
	fbConfigTempFiles, err := readFbConfigTempFilesFromTempDirectory(tempDir)
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

	var configTempFilesToRemove []string
	var removedConfigTempFiles []string
	var listErrors listError

	// create list of fbConfigTempFiles to remove
	for i := 0; i < len(fbConfigTempFiles)-maxNumberOfFbConfigTempFiles; i++ {
		configTempFilesToRemove = append(configTempFilesToRemove, fbConfigTempFiles[i].Name())
	}

	// extract lua filter filenames from config temp files to remove
	for _, configTempFileToRemove := range configTempFilesToRemove {
		if fbLuaFilterTempFilenames, err := extractLuaFilterFilenames(tempDir, configTempFileToRemove); err != nil {
			listErrors.Add(err)
		} else {
			configTempFilesToRemove = append(configTempFilesToRemove, fbLuaFilterTempFilenames...)
		}
	}

	// remove all config and lua filter temp files from temporary directory
	for _, configTempFileToRemove := range configTempFilesToRemove {
		if err := os.Remove(filepath.Join(tempDir, configTempFileToRemove)); err != nil {
			listErrors.Add(err)
		} else {
			removedConfigTempFiles = append(removedConfigTempFiles, configTempFileToRemove)
		}
	}

	return removedConfigTempFiles, listErrors.ErrorOrNil()
}

// return the list of temp config files from temporary directory.
func readFbConfigTempFilesFromTempDirectory(tempDir string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(tempDir)
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
func extractLuaFilterFilenames(tempDir string, fbConfigTempFilename string) ([]string, error) {
	fbConfigTempFileContent, err := os.ReadFile(filepath.Join(tempDir, fbConfigTempFilename))
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
