// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/pkg/config/loader"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

// PluginsToml represents the data model for the a plugin configuration file, listing one or more plugins and configuration for them
type PluginsToml struct {
	PluginConfigs []PluginConfig `yaml:"plugin"`
}

// PluginConfig represents configuration for a single instance of a running plugin
type PluginConfig struct {
	PluginName      string              `yaml:"name"`
	PluginInstances []map[string]string `yaml:"instance"`
}

// LoadPluginConfig reads the given config file and parses it into a PluginsToml object specifying configuration of plugins to use
func LoadPluginConfig(registry *PluginRegistry, configFiles []string) (*PluginsToml, error) {
	pluginsToml := &PluginsToml{}

	configFilePaths := []string{}
	for _, configFile := range configFiles {
		configFile, err := filepath.Abs(configFile)
		if err != nil {
			return nil, err
		}
		configFilePaths = append(configFilePaths, configFile)
	}
	_, err := config_loader.LoadYamlConfig(pluginsToml, configFilePaths...)
	if err != nil {
		return nil, err
	}

	if len(pluginsToml.PluginConfigs) == 0 {
		log.WithFields(logrus.Fields{
			"action":      "LoadPluginConfig",
			"configFiles": configFiles,
		}).Debug("No plugin configuration found. Using default configuration for all available plugins.")
	}

	// To minimize required configuration, create default configuration for all plugins we have
	// in the plugins directory if the user has not specified any configuration for them.
	// (This can only be done for plugins not requiring configuration)
	for pluginName, plugin := range registry.plugins {
		if plugin.ProtocolVersion >= protocol.V1 {
			continue
		}

		if len(plugin.Properties) == 0 {
			found := false
			for _, config := range pluginsToml.PluginConfigs {
				if config.PluginName == pluginName {
					found = true
					break
				}
			}

			if !found {
				// No user-defined configuration for this plugin. Create a default instance.
				pluginsToml.PluginConfigs = append(pluginsToml.PluginConfigs, PluginConfig{
					PluginName: pluginName,
				})
			}
		}
	}

	return pluginsToml, nil
}
