// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/config/loader"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var plog = log.WithComponent("PluginRegistry")

type PluginRegistry struct {
	plugins            map[string]*Plugin // Map of plugin name to loaded plugins
	pluginInstances    []*PluginV1Instance
	pluginSourceDirs   []string // Directories where plugins are stored
	pluginInstanceDirs []string // Directories where plugin instances are configured
}

func NewPluginRegistry(pluginSourceDirs, pluginInstanceDirs []string) *PluginRegistry {
	return &PluginRegistry{
		plugins:            make(map[string]*Plugin),
		pluginSourceDirs:   pluginSourceDirs,
		pluginInstanceDirs: pluginInstanceDirs,
	}
}

func (pr *PluginRegistry) GetPluginInstances() []*PluginV1Instance {
	return pr.pluginInstances
}

func (pr *PluginRegistry) GetPlugin(pluginName string) (*Plugin, error) {
	if plugin, ok := pr.plugins[pluginName]; ok {
		return plugin, nil
	} else {
		return nil, errors.New("Integration definition not found")
	}
}

// GetPluginDir gets the plugin directory so the plugin runner knows where to run the command
func (pr *PluginRegistry) GetPluginDir(plugin *Plugin) string {
	pluginDir := plugin.workingDir
	if info, err := os.Stat(pluginDir); err == nil && info.IsDir() {
		return pluginDir
	}
	plog.WithField("plugin", plugin.Name).Debug("Returning Empty Plugin Directory.")
	return ""
}

func (pr *PluginRegistry) LoadPlugins() (err error) {
	err = pr.LoadPluginSources()
	if err == nil {
		err = pr.LoadPluginInstances()
	}
	return err
}

func (pr *PluginRegistry) LoadPluginInstances() (err error) {
	for _, pluginInstanceDir := range pr.pluginInstanceDirs {
		if pluginInstanceDir == "" {
			continue
		}
		pluginInstanceDirs, err := ioutil.ReadDir(pluginInstanceDir)
		if err != nil {
			if os.IsNotExist(err) {
				// The plugin instances directory doesn't exist, so skip processing it.
				// Order counts here, and override directories are being scanned first
				continue
			}
			return err
		}

		for _, pluginInstanceDirOrFile := range pluginInstanceDirs {
			pr.loadPluginInstance(pluginInstanceDir, pluginInstanceDirOrFile)
		}
	}
	return nil
}

func (pr *PluginRegistry) loadPluginInstance(dir string, dirOrFile os.FileInfo) {
	dirOrFileName := dirOrFile.Name()
	dirOrFilePath := filepath.Join(dir, dirOrFileName)

	// Ignore non yaml files or directories
	fileExt := filepath.Ext(dirOrFileName)
	if fileExt != ".yaml" && fileExt != ".yml" {
		plog.WithField("path", dirOrFilePath).
			Debug("Ignoring directory or non yaml integration config file.")
		return
	}

	instanceWrapper, err := pr.loadPluginInstanceWrapper(dirOrFilePath)
	if err != nil {
		plog.WithField("configFile", dirOrFilePath).
			Error("Couldn't load integration config file")
		return
	}

	// ignore V4 plugins
	if instanceWrapper.IntegrationName == "" && len(instanceWrapper.Instances) == 0 {
		plog.WithField("file", dirOrFilePath).
			Debug("Ignoring v4 integration. To be loaded later.")
		return
	}

	pilog := plog.WithFields(logrus.Fields{
		"integration": instanceWrapper.IntegrationName,
		"configFile":  dirOrFileName,
	})
	plugin, err := pr.GetPlugin(instanceWrapper.IntegrationName)
	if err != nil {
		pilog.WithError(err).Error("Couldn't load integration instances from config file")
		return
	}

	// If data binding is enabled, builds the data sources to apply them later
	if instanceWrapper.DataBind.Enabled() {
		pilog.Debug("Instantiating Databind sources.")
		var err error
		plugin.discovery, err = databind.DataSources(&instanceWrapper.DataBind)
		if err != nil {
			pilog.WithError(err).Error("variables/discovery data binding problem. Ignoring this plugin")
			return
		}
	}

	for _, instance := range instanceWrapper.Instances {
		if _, ok := plugin.Commands[instance.Command]; !ok {
			pilog.WithField("command", instance.Command).
				Error("Integration instance command not found in definition")
			continue
		}
		instance.plugin = plugin
		pr.pluginInstances = append(pr.pluginInstances, instance)
	}
}

func (pr *PluginRegistry) loadPluginInstanceWrapper(pluginInstanceFilePath string) (pluginInstanceWrapper *PluginInstanceWrapper, err error) {
	plog.WithField("configFile", pluginInstanceFilePath).Debug("Found integration config file.")
	var decodedPluginInstanceWrapper PluginInstanceWrapper
	if _, err = config_loader.LoadYamlConfig(&decodedPluginInstanceWrapper, pluginInstanceFilePath); err != nil {
		return
	}
	if _, err = config_loader.LoadYamlConfig(&decodedPluginInstanceWrapper.DataBind, pluginInstanceFilePath); err != nil {
		return
	}
	return &decodedPluginInstanceWrapper, nil
}

// LoadPlugins scans all plugins in the plugin storage dir and loads their
// source details into the registry. Maintains backward compatible
func (pr *PluginRegistry) LoadPluginSources() (err error) {

	for _, pluginSourceDir := range pr.pluginSourceDirs {
		if pluginSourceDir == "" {
			continue
		}
		pluginDirs, err := ioutil.ReadDir(pluginSourceDir)
		if err != nil {
			if os.IsNotExist(err) {
				// The plugin directory doesn't exist, so skip processing it.
				// Order counts here, and override directories are being scanned first
				continue
			}
			return err
		}

		for _, pluginDirOrFile := range pluginDirs {

			pluginDirOrFileName := pluginDirOrFile.Name()
			fullPluginDirOrFilePath := filepath.Join(pluginSourceDir, pluginDirOrFileName)

			// Load either Legacy or V1 plugins' config (any yaml file), skip everything else
			var plugin *Plugin
			if pluginDirOrFile.IsDir() {
				if plugin, err = pr.loadLegacyPlugin(fullPluginDirOrFilePath); err != nil {
					plog.WithField("path", fullPluginDirOrFilePath).WithError(err).
						Error("Loading integrations from directory")
					continue
				} else if plugin == nil {
					// We should skip this directory, no plugin.yaml was present
					plog.WithField("path", fullPluginDirOrFilePath).
						Debug("Tried to load an integration where no plugin.yaml was present.")
					continue
				}
				// Working directory in V0 is the plugin's folder
				plugin.workingDir = fullPluginDirOrFilePath
			} else if filepath.Ext(pluginDirOrFileName) == ".yaml" || filepath.Ext(pluginDirOrFileName) == ".yml" {
				// See if this is a symlink to the yaml file, and swap that path here
				resolvedPath := fullPluginDirOrFilePath
				if pluginDirOrFile.Mode()&os.ModeSymlink == os.ModeSymlink {
					resolvedPath, err = filepath.EvalSymlinks(fullPluginDirOrFilePath)
					if err != nil {
						plog.WithField("path", fullPluginDirOrFilePath).WithError(err).
							Error("Unable to resolve integration definition symlink")
						continue
					}
				}
				if plugin, err = pr.loadPlugin(resolvedPath); err != nil || plugin == nil {
					plog.WithField("path", resolvedPath).WithError(err).
						Error("Loading integration definition")
					continue
				}
				// Working directory in V1 is config file's directory, after symlink resolution
				plugin.workingDir = filepath.Dir(resolvedPath)
			} else {
				continue
			}

			// Check OS support information V1 plugins
			if plugin.OS == "" || strings.Contains(strings.ToLower(plugin.OS), runtime.GOOS) {
				plog.WithField("integration", plugin.Name).Debug("Integration definition loaded.")
				pr.plugins[plugin.Name] = plugin
			} else {
				plog.WithFields(logrus.Fields{
					"integration": plugin.Name,
					"currentOS":   runtime.GOOS,
					"requiredOS":  plugin.OS,
				}).Debug("Ignoring integration, unsupported operating system.")
			}
		}
	}
	return
}

func (pr *PluginRegistry) loadLegacyPlugin(pluginDir string) (plugin *Plugin, err error) {
	pluginPath := filepath.Join(pluginDir, "plugin.yaml")
	if _, err = os.Stat(pluginPath); os.IsNotExist(err) {
		return nil, nil
	}

	return pr.loadPlugin(pluginPath)
}

func isInitialized(p ids.PluginID) bool {
	return p.Term != ""
}

func (pr *PluginRegistry) loadPlugin(pluginPath string) (plugin *Plugin, err error) {
	plog.WithField("definitionFile", pluginPath).Debug("Found integration definition file.")
	var decodedPlugin Plugin
	if _, err = config_loader.LoadYamlConfig(&decodedPlugin, pluginPath); err != nil {
		return
	}
	plugin = &decodedPlugin

	// Prefix is an optional field an a default is provided
	for commandName, command := range plugin.Commands {
		if !isInitialized(command.Prefix) {
			command.Prefix = ids.PluginID{Category: V1_DEFAULT_PLUGIN_CATEGORY, Term: plugin.Name}
			plog.WithFields(logrus.Fields{
				"integration": plugin.Name,
				"command":     command.Command,
				"prefix":      command.Prefix,
			}).Debug("Integration prefix not specified in definition, using default.")
		}

		commandLine := command.Command

		if len(commandLine) == 0 {
			plog.WithFields(logrus.Fields{
				"integration": plugin.Name,
				"command":     command.Command,
				"commandName": commandName,
				"prefix":      command.Prefix,
			}).Error("Missing command attribute in definition, skipping integration")
			return nil, nil
		}
		command.Command = commandLine
	}

	return
}
