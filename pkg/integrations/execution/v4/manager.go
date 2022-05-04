// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	"context"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/config/migrate"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	config_v32 "github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v3/config"
	config2 "github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/fs"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/emitter"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/newrelic/infrastructure-agent/pkg/config/envvar"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/constants"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/files"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/runner"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var illog = log.WithComponent("integrations.Manager")

const (
	// Integrations V3 configs use the "instances" key word
	// In the current Integrations config, we use "integrations"
	// The two fields below will allows us distinguish between both, to ignore V4
	integrationsField    = "integrations"
	LegacyInstancesField = "instances"
)

// not an actual error. Used for discarding V3 plugins
var legacyYAML = errors.New("file format belongs to the old integrations format")

// runner-groups contexts indexed per config path, bundling lock to support concurrent access.
type rgsPerPath struct {
	l sync.RWMutex
	m map[string]*groupContext
}

func newRunnerGroupsPerCfgPath() *rgsPerPath {
	return &rgsPerPath{
		l: sync.RWMutex{},
		m: make(map[string]*groupContext),
	}
}

func (r *rgsPerPath) List() map[string]*groupContext {
	r.l.RLock()
	defer r.l.RUnlock()

	// avoid concurrent map access
	aux := make(map[string]*groupContext)
	for k, v := range r.m {
		aux[k] = v
	}

	return aux
}

func (r *rgsPerPath) Set(cfgPath string, rc *groupContext) {
	r.l.Lock()
	defer r.l.Unlock()

	r.m[cfgPath] = rc
}

func (r *rgsPerPath) Get(cfgPath string) (rg *groupContext, exists bool) {
	r.l.RLock()
	defer r.l.RUnlock()

	rg, exists = r.m[cfgPath]
	return
}

func (r *rgsPerPath) Remove(cfgPath string) {
	r.l.Lock()
	defer r.l.Unlock()

	delete(r.m, cfgPath)
}

func (r *rgsPerPath) isGroupRunning(cfgPath string) bool {
	if ctx, ok := r.Get(cfgPath); ok && ctx != nil {
		return ctx.isRunning()
	}

	return false
}

type Manager struct {
	config                   Configuration
	watcher                  *fsnotify.Watcher
	runners                  *rgsPerPath
	emitter                  emitter.Emitter
	lookup                   integration.InstancesLookup
	featuresCache            runner.FeaturesCache
	definitionQueue          <-chan integration.Definition
	configEntryQueue         <-chan configrequest.Entry
	terminateDefinitionQueue chan string
	handleCmdReq             cmdrequest.HandleFn
	handleConfig             configrequest.HandleFn
	tracker                  *track.Tracker
	idLookup                 host.IDLookup
	pluginRegistry           *config_v32.PluginRegistry
}

// groupContext pairs a runner.Group with its cancellation context
type groupContext struct {
	l      sync.RWMutex
	cancel func() // nil when there's no cancellable context
	runner runner.Group
}

func newGroupContext(gr runner.Group) *groupContext {
	return &groupContext{
		runner: gr,
		l:      sync.RWMutex{},
	}
}

func (g *groupContext) start(ctx context.Context) {
	g.l.Lock()
	defer g.l.Unlock()

	cctx, cancel := context.WithCancel(ctx)
	if g.runner.Run(cctx) {
		g.cancel = cancel
	}
}

func (g *groupContext) stop() {
	g.l.Lock()
	defer g.l.Unlock()

	if g.cancel != nil {
		g.cancel()
	}
}

func (g *groupContext) isRunning() bool {
	g.l.RLock()
	defer g.l.RUnlock()

	return g.cancel != nil
}

type Configuration struct {
	// Configfolders store the YAML integrations configurations.
	// They may also contain -config.yml files from v3 integrations
	ConfigFolders []string
	AgentFeatures map[string]bool // features from agent config file
	// DefinitionFolders store the v3 -definition.yml plugins (legacy support)
	// and the executables where the agent will look for if only the 'name' property is specified for an integration
	DefinitionFolders []string
	// Defines verbosity level in v3 legacy integrations
	Verbose int
	// PassthroughEnvironment holds a copy of its homonym in config.Config.
	PassthroughEnvironment []string
}

func NewConfig(verbose int, features map[string]bool, passthroughEnvs, configFolders, definitionFolders []string) Configuration {
	return Configuration{
		ConfigFolders:          configFolders,
		AgentFeatures:          features,
		DefinitionFolders:      definitionFolders,
		Verbose:                verbose,
		PassthroughEnvironment: passthroughEnvs,
	}
}

// NewManager loads all the integration configuration files from the given folders. It discards the integrations
// not belonging to the protocol V4.
// Usually, "configFolders" will be the value of the "pluginInstanceDir" configuration option
// The "definitionFolders" refer to the v3 definition yaml configs, placed here for v3 integrations backwards-support
func NewManager(
	cfg Configuration,
	emitter emitter.Emitter,
	il integration.InstancesLookup,
	definitionQ chan integration.Definition,
	terminateDefinitionQ chan string,
	configEntryQ chan configrequest.Entry,
	tracker *track.Tracker,
	idLookup host.IDLookup,
	pluginRegistry *config_v32.PluginRegistry,
) *Manager {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		illog.WithError(err).Warn("can't enable hot reload")
	}

	mgr := Manager{
		config:                   cfg,
		runners:                  newRunnerGroupsPerCfgPath(),
		emitter:                  emitter,
		watcher:                  watcher,
		lookup:                   il,
		featuresCache:            make(runner.FeaturesCache),
		definitionQueue:          definitionQ,
		configEntryQueue:         configEntryQ,
		terminateDefinitionQueue: terminateDefinitionQ,
		handleCmdReq:             cmdrequest.NewHandleFn(definitionQ, il, illog),
		handleConfig:             configrequest.NewHandleFn(configEntryQ, terminateDefinitionQ, il, illog),
		tracker:                  tracker,
		idLookup:                 idLookup,
		pluginRegistry:           pluginRegistry,
	}

	// Loads all the configuration files in the passed configFolders
	for _, folder := range cfg.ConfigFolders {
		flog := illog.WithField("folder", folder)

		configs, err := mgr.configFilesIn(folder)
		if err != nil {
			elog := flog.WithError(err)
			if os.IsNotExist(err) {
				elog.Debug("Directory does not exist. Ignoring.")
			} else {
				elog.Warn("can't load directory. Ignoring")
			}
			continue
		}

		if watcher != nil {
			flog.Debugf("watching %v", folder)
			if err := watcher.Add(folder); err != nil {
				flog.WithError(err).Warn("cant watch for file changes in folder")
			}
			for i := range configs {
				flog.Debugf("watching :%v", i)
				if err := watcher.Add(i); err != nil {
					flog.WithError(err).Warn("cant watch for file change")
				}
			}
		}

		flog.WithFieldsF(foundFilesLogFields(configs)).Debug("Loading integrations from folder.")

		mgr.loadEnabledRunnerGroups(configs)
	}

	return &mgr
}

// Start in background the v4 integrations lifecycle management, including hot reloading, interval and timeout management
func (mgr *Manager) Start(ctx context.Context) {
	for path, rc := range mgr.runners.List() {
		illog.WithField("file", path).Debug("Starting integrations group.")
		rc.start(contextWithVerbose(ctx, mgr.config.Verbose))
	}

	go mgr.handleRequestsQueue(ctx)

	mgr.watchForFSChanges(ctx)
}

// EnableOHIFromFF enables an integration coming from CC request.
func (mgr *Manager) EnableOHIFromFF(ctx context.Context, featureFlag string) error {
	cfgPath, err := mgr.cfgPathForFF(featureFlag)
	if err != nil {
		return err
	}

	if mgr.runners.isGroupRunning(cfgPath) {
		return nil
	}

	cmdFF := runner.CmdFF{
		Name:    featureFlag,
		Enabled: true,
	}

	mgr.runIntegrationFromPath(ctx, cfgPath, false, &illog, &cmdFF)

	return nil
}

// DisableOHIFromFF disables an integration coming from CC request.
// Formats btw CC FF and config files: see EnableOHIFromCmd
func (mgr *Manager) DisableOHIFromFF(featureFlag string) error {
	cfgPath, err := mgr.cfgPathForFF(featureFlag)
	if err != nil {
		return err
	}

	mgr.stopRunnerGroup(cfgPath)

	return nil
}

func (mgr *Manager) loadEnabledRunnerGroups(cfgs map[string]config2.YAML) {
	for path, cfg := range cfgs {
		if rc, err := mgr.loadRunnerGroup(path, cfg, nil); err != nil {
			illog.WithField("file", path).WithError(err).Warn("can't instantiate integrations from file")
		} else {
			mgr.runners.Set(path, rc)
		}
	}
}

func (mgr *Manager) loadRunnerGroup(path string, cfg config2.YAML, cmdFF *runner.CmdFF) (*groupContext, error) {
	f := runner.NewFeatures(mgr.config.AgentFeatures, cmdFF)
	loader := runner.NewLoadFn(cfg, f)
	gr, fc, err := runner.NewGroup(loader, mgr.lookup, mgr.config.PassthroughEnvironment, mgr.emitter, mgr.handleCmdReq, mgr.handleConfig, path, mgr.terminateDefinitionQueue, mgr.idLookup)
	if err != nil {
		return nil, err
	}

	mgr.featuresCache.Update(fc)

	return newGroupContext(gr), nil
}

func (mgr *Manager) handleRequestsQueue(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case def := <-mgr.definitionQueue:
			r := runner.NewRunner(def, mgr.emitter, nil, nil, mgr.handleCmdReq, nil, mgr.terminateDefinitionQueue, mgr.idLookup)
			if def.CmdChanReq != nil {
				// tracking so cmd requests can be stopped by hash
				runCtx, pidWCh := mgr.tracker.Track(ctx, def.CmdChanReq.CmdChannelCmdHash, &def)
				go func(hash string) {
					exitCodeCh := make(chan int, 1)
					r.Run(runCtx, pidWCh, exitCodeCh)
					mgr.tracker.NotifyExit(hash, <-exitCodeCh)
					mgr.tracker.Untrack(hash)
				}(def.CmdChanReq.CmdChannelCmdHash)
			} else {
				go r.Run(ctx, nil, nil)
			}
		case entry := <-mgr.configEntryQueue:
			ds, _ := entry.Databind.DataSources()
			r := runner.NewRunner(entry.Definition, mgr.emitter, ds, nil, nil, nil, mgr.terminateDefinitionQueue, mgr.idLookup)
			runCtx, pidWCh := mgr.tracker.Track(ctx, entry.Definition.Hash(), &entry.Definition)
			go r.Run(runCtx, pidWCh, nil)

		case entry := <-mgr.terminateDefinitionQueue:
			mgr.tracker.Kill(entry)
		}
	}
}

// watch for changes in the plugins directories and loads/cancels/reloads the affected integrations
func (mgr *Manager) watchForFSChanges(ctx context.Context) {
	if mgr.watcher == nil {
		return
	}

	wclog := illog.WithField("function", "watchForChanges")
	wclog.Debug("Watching for integrations file changes.")
	for {
		select {
		case <-ctx.Done():
			wclog.Debug("Integration manager context cancelled. Stopped watching for file changes.")
			return

		case event := <-mgr.watcher.Events:
			mgr.handleFileEvent(ctx, &event)

		case err := <-mgr.watcher.Errors:
			wclog.WithError(err).Debug("Error watching file changes.")
		}
	}
}

func (mgr *Manager) handleFileEvent(ctx context.Context, event *fsnotify.Event) {
	wclog := illog.WithField("function", "handleFileEvent")

	if event == nil {
		wclog.Debug("Unexpected nil watcher event. Ignoring.")
		return
	}
	elog := wclog.
		WithField("event", event.String()).
		WithField("file_name", event.Name)
	elog.Debug("Received File event.")

	var eDelete, eCreate, eWrite, eRename bool
	if event.Op&fsnotify.Write == fsnotify.Write {
		eWrite = true
	}
	if event.Op&fsnotify.Remove == fsnotify.Remove {
		eDelete = true
	}
	if event.Op&fsnotify.Create == fsnotify.Create {
		eCreate = true
	}
	if event.Op&fsnotify.Rename == fsnotify.Rename {
		eRename = true
	}

	isDelete := eDelete || eRename
	isCreate := eCreate
	isWrite := isCreate || eWrite
	if !isDelete && !isWrite {
		elog.Debug("Ignoring File event.")
		return
	}

	if event.Name == "" {
		elog.Debug("File event name is empty. Ignoring.")
		return
	}
	if err := fs.ValidateYAMLFile(event.Name, isDelete); err != nil {
		illog.WithField("file", event.Name).WithError(err).
			Debug("Not an existing YAML file. Ignoring.")
		return
	}

	mgr.stopRunnerGroup(event.Name)

	if isDelete {
		if _, err := os.Stat(event.Name); os.IsNotExist(err) {
			// if the file has been deleted, we don't continue trying to load configurations
			return
		}

		elog.Debugf("file '%v' says deleted but still here", event.Name)
		if err := mgr.watcher.Add(event.Name); err != nil {
			elog.WithError(err).Warn("cant watch for file changes")
		}
	}

	if isCreate {
		elog.Debugf("watching file '%v' as brand new", event.Name)
		if err := mgr.watcher.Add(event.Name); err != nil {
			elog.WithError(err).Warn("cant watch for file changes")
		}

	}
	// creating new configuration and starting the new runner.Group instances
	mgr.runIntegrationFromPath(ctx, event.Name, isCreate, &elog, nil)
}

func (mgr *Manager) runIntegrationFromPath(ctx context.Context, cfgPath string, isCreate bool, elog *log.Entry, cmdFF *runner.CmdFF) {
	cfg, err := mgr.loadConfig(cfgPath)
	if err != nil {
		if err == legacyYAML {
			elog.Debug("Skipping v3 integration.")
		} else {
			elog.WithError(err).Warn("can't load integrations file. This may happen if you are editing a file and saving intermediate changes")
		}
		return
	}

	if isCreate {
		elog.Debug("New integration file has been created.")
	}

	rc, err := mgr.loadRunnerGroup(cfgPath, cfg, cmdFF)
	if err != nil {
		elog.WithError(err).Warn("can't instantiate integrations from file. This may happen if you are editing a file and saving intermediate changes")
		return
	}

	mgr.runners.Set(cfgPath, rc)
	rc.start(ctx)
}

func (mgr *Manager) stopRunnerGroup(fileName string) {
	if ctx, ok := mgr.runners.Get(fileName); ok && ctx != nil && ctx.isRunning() {
		illog.WithField("file", fileName).
			Info("integration file modified or deleted. Stopping running processes, if any")
		ctx.stop()
		mgr.runners.Remove(fileName)
	}
}

// featureName is the OHI config "feature" value. ie: feature: docker
func (mgr *Manager) cfgPathForFF(featureName string) (cfgPath string, err error) {
	cfgPath, ok := mgr.featuresCache[featureName]
	if !ok {
		err = errors.New("cannot find cfg file for feature")
		return
	}

	return
}

// reads the configuration files in a given folder, and discards those not belonging to the V4 format
func (mgr *Manager) configFilesIn(folder string) (map[string]config2.YAML, error) {
	cflog := illog.WithField("folder", folder)

	yamlFiles, err := files.AllYAMLs(folder)
	if err != nil {
		return nil, err
	}

	configs := map[string]config2.YAML{}
	for _, file := range yamlFiles {
		flog := cflog.WithField("file", file.Name())
		absolutePath := filepath.Join(folder, file.Name())
		flog.Debug("Loading config.")
		cfg, err := mgr.loadConfig(absolutePath)
		if err != nil {
			if err == legacyYAML {
				flog.Debug("Skipping v3 integration. To be loaded in another moment.")
			} else {
				flog.WithError(err).Warn("can't load integrations file")
			}
			continue
		}
		configs[absolutePath] = cfg
	}
	return configs, nil
}

func (mgr *Manager) loadConfigIntoBytes(path string) ([]byte, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return bytes, nil
	}

	err = explainEmptyIntegrations(bytes)
	if err == legacyYAML {
		v3Configuration := config_v32.PluginInstanceWrapper{}
		err = migrate.ReadAndUnmarshallConfig(path, &v3Configuration)
		if err != nil {
			return nil, fmt.Errorf("error reading old config configuration: %w", err)
		}

		// Reading old Definition file
		v3Definition, err := mgr.pluginRegistry.GetPlugin(v3Configuration.IntegrationName)
		if err != nil {
			return nil, fmt.Errorf("error reading old config configuration: %w", err)
		}
		v4Conf, err := migrate.PopulateV4Config(*v3Definition, v3Configuration)
		if err != nil {
			return nil, fmt.Errorf("error reading old config configuration: %w", err)
		}
		bytes, err = yaml.Marshal(v4Conf)
		if err != nil {
			return nil, fmt.Errorf("error reading old config configuration: %w", err)
		}
	}

	return bytes, nil
}

func (mgr *Manager) loadConfig(path string) (config2.YAML, error) {
	bytes, err := mgr.loadConfigIntoBytes(path)
	cy := config2.YAML{}
	if err != nil {
		return cy, err
	}

	bytes, err = envvar.ExpandInContent(bytes)
	if err != nil {
		return cy, err
	}

	if err := yaml.Unmarshal(bytes, &cy); err != nil {
		return cy, err
	}
	if len(cy.Integrations) == 0 {
		return cy, explainEmptyIntegrations(bytes)
	}
	return cy, nil
}

// returns why a v4 integration is empty: because it's a v3 integration or because it has a wrong format
func explainEmptyIntegrations(bytes []byte) error {
	var contents map[string]interface{}
	err := yaml.Unmarshal(bytes, &contents)
	if err != nil {
		return err // should never happen
	}
	if _, ok := contents[LegacyInstancesField]; ok {
		return legacyYAML
	}
	if _, ok := contents[integrationsField]; ok {
		return errors.New("'" + integrationsField + "' seems to be empty or wrongly formatted")
	}
	return errors.New("missing '" + integrationsField + "' field")
}

// auxiliary logger fields provider function
func foundFilesLogFields(configs map[string]config2.YAML) func() logrus.Fields {
	return func() logrus.Fields {
		var found string
		if len(configs) == 0 {
			found = "none"
		} else {
			fs := make([]string, 0, len(configs))
			for path := range configs {
				fs = append(fs, filepath.Base(path))
			}
			found = strings.Join(fs, ", ")
		}
		return logrus.Fields{"found": found}
	}
}

func contextWithVerbose(ctx context.Context, verbose int) context.Context {
	return context.WithValue(ctx, constants.EnableVerbose, verbose)
}
