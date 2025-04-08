// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	"errors"
	"io/ioutil"
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/fs"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	"gopkg.in/yaml.v2"
)

var loaderLogger = log.WithComponent("integrations.Supervisor.Loader").WithField("process", "log-forwarder")

const (
	fluentBitTagTroubleshoot = "nri-troubleshoot"
)

type CfgLoader struct {
	config           config.LogForward
	loadFilesFn      fs.FilesInFolderFn
	agentIDFn        id.Provide
	hostnameResolver hostname.Resolver
}

func NewFolderLoader(c config.LogForward, agentIDFn id.Provide, hostnameResolver hostname.Resolver) *CfgLoader {
	return &CfgLoader{
		config:           c,
		loadFilesFn:      fs.OSFilesInFolderFn,
		agentIDFn:        agentIDFn,
		hostnameResolver: hostnameResolver,
	}
}

func (l *CfgLoader) GetConfigDir() string {
	return l.config.ConfigsDir
}

func (l *CfgLoader) GetLicenseKey() string {
	return l.config.License
}

// LoadAll loads and parses the logging configuration. It returns ok=false in case an error occurred, which should block
// the start of the log forwarding feature.
func (l *CfgLoader) LoadAll(ff feature_flags.Retriever) (c FBCfg, ok bool) {
	if l.config.ConfigsDir == "" && !l.config.Troubleshoot.Enabled {
		loaderLogger.Error("invalid config, lacking config folder or troubleshoot mode")
		return FBCfg{}, false
	}

	allFilesCfgs, ok := l.loadFolderCfgs()
	if !ok {
		return FBCfg{}, false
	}

	if t := l.loadTroubleshootCfg(); t != nil {
		allFilesCfgs = append(allFilesCfgs, *t)
	}

	if len(allFilesCfgs) == 0 {
		loaderLogger.Debug("Could not find any configuration for logging forwarder.")
		return FBCfg{}, false
	}

	// single FluentBit instance config for all logs in all files
	agentGUID := l.agentIDFn().GUID // blocks until ID is available
	_, shortHostName, err := l.hostnameResolver.Query()
	if err != nil {
		loaderLogger.Debug("Could not determine hostname.")
	}

	c, err = NewFBConf(allFilesCfgs, &(l.config), agentGUID.String(), shortHostName, ff)
	if err != nil {
		loaderLogger.WithError(err).Error("could not process logging configurations")
		return FBCfg{}, false
	}

	return
}

// loadFolderCfgs loads all YAML logging configuration files from the logging configuration folder and parses them
// into a slice of LogCfg (LogsCfg). It returns ok=true upon success, or ok=false in case that an error occurred while
// loading any of the files, or if no valid configurations were found.
func (l *CfgLoader) loadFolderCfgs() (cfgs LogsCfg, ok bool) {
	var files []string
	var err error
	if l.config.ConfigsDir != "" {
		files, err = l.loadFilesFn(l.config.ConfigsDir)
		if err != nil && err != fs.ErrFilesNotFound {
			loaderLogger.WithError(err).Error("could not load files within the configuration directory")
			return nil, false
		}
	}

	ok = true
	for _, f := range files {
		fileCfgs, okFile := l.loadFileCfgs(f)
		if !okFile {
			ok = false
		}

		cfgs = append(cfgs, fileCfgs...)
	}
	return cfgs, ok
}

// loadFileCfgs loads the logging configurations present in a single file. It returns ok=true upon success, or ok=false
// in case that an error occurred while reading or parsing the file.
func (l *CfgLoader) loadFileCfgs(file string) (cfgs LogsCfg, ok bool) {
	// Only consider configuration files in YAML format (*.yml or *.yaml)
	if ext := filepath.Ext(file); ext != ".yml" && ext != ".yaml" {
		loaderLogger.WithField("file", file).WithField("extension", ext).Debug("Ignoring file due to non-YAML extension.")
		return nil, true
	}

	content, err := ioutil.ReadFile(file)
	if err != nil {
		loaderLogger.WithError(err).WithField("file", file).Error("cannot read file")
		return nil, false
	}

	// each file may contain several log entries
	fileCfgs, err := l.parseYAML(content)
	if err != nil {
		loaderLogger.WithError(err).WithField("file", file).Error("could not parse YAML file")
		return nil, false
	}

	// empty config could be returned if there is a file with no valid config
	if len(fileCfgs) == 0 {
		loaderLogger.WithField("file", file).Debug("No configurations found in file.")
	}

	return fileCfgs, true
}

// loadTroubleshootCfg returns, in case the Troubleshoot mode is enabled, a logging configuration targeted to capture
// the infra-agent logs.
func (l *CfgLoader) loadTroubleshootCfg() *LogCfg {
	if l.config.Troubleshoot.Enabled {
		var aLog LogCfg
		if l.config.Troubleshoot.AgentLogPath != "" {
			aLog = LogCfg{
				Name: fluentBitTagTroubleshoot,
				File: l.config.Troubleshoot.AgentLogPath,
			}
		} else {
			aLog = LogCfg{
				Name:    fluentBitTagTroubleshoot,
				Systemd: "newrelic-infra", // agent service name
			}
		}
		return &aLog
	}
	return nil
}

func (l *CfgLoader) LoadAndFormat(ff feature_flags.Retriever) (string, FBCfgExternal, error) {
	fbConfig, ok := l.LoadAll(ff)
	if !ok {
		return "", FBCfgExternal{}, errors.New("failed to load log configs")
	}
	return fbConfig.Format()
}

func (l *CfgLoader) parseYAML(content []byte) (c LogsCfg, err error) {
	var y YAML
	if err = yaml.Unmarshal(content, &y); err != nil {
		return
	}

	// prevent inconsistent data
	if len(y.Logs) == 0 {
		return
	}

	for _, cfg := range y.Logs {
		if cfg.IsValid() {
			c = append(c, cfg)
		}
	}

	return
}

// hot config reload api proposal
//func (l *CfgLoader) ListenForUpdates(ctx context.Context) <- chan FBCfg {
//	return
//}
