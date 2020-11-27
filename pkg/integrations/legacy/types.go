// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

//==============================================================================
// Model for definition of plugins
//==============================================================================

// Plugin represents a single plugin, with all associated metadata
type Plugin struct {
	Name            string                      `yaml:"name"`             // Name of the plugin (required)
	Description     string                      `yaml:"description"`      // A short plugin description (optional)
	Commands        map[string]*PluginV1Command `yaml:"commands"`         // Map of commands for v1 plugins
	Sources         []*PluginSource             `yaml:"source"`           // List of sources to execute for the plugin to gather data.
	OS              string                      `yaml:"os"`               // OS (or comma-separated list of OSes) supported for the plugin
	Properties      []PluginProperty            `yaml:"property"`         // Properties to control behavior of this plugin
	ProtocolVersion int                         `yaml:"protocol_version"` // Protocol version (0 == original version)
	workingDir      string
	discovery       *databind.Sources
}

const (
	V1_DEFAULT_PLUGIN_CATEGORY = "integration"
	V1_DEFAULT_EVENT_CATEGORY  = "notifications"
	V1_REQUIRED_EVENT_FIELD    = "summary"
	V1_EVENT_EVENT_TYPE        = "InfrastructureEvent"
)

type PluginV1Command struct {
	Command  []string     `yaml:"command"`  // Command to execute, run from the plugin's directory.
	Prefix   ids.PluginID `yaml:"prefix"`   // "Plugin path" for inventory data produced by the plugin. Not applicable for event sources.
	Interval int          `yaml:"interval"` // Number of seconds to wait between invocations of the source.
}

type PluginV1Instance struct {
	Name      string            `yaml:"name"`
	Command   string            `yaml:"command"`
	Arguments map[string]string `yaml:"arguments"`
	Labels    map[string]string `yaml:"labels"`
	// System user for running the integration, if set to something different
	// than "" the integration binary will be executed as
	// `sudo -n -u <become> <integration_binary> <args>`
	IntegrationUser string  `yaml:"integration_user"`
	plugin          *Plugin `yaml:"-"`
}

type PluginInstanceWrapper struct {
	IntegrationName string              `yaml:"integration_name"`
	Instances       []*PluginV1Instance `yaml:"instances"`
	DataBind        databind.YAMLConfig `yaml:"-"` // binding of fetched variables and discovery data
}

// PluginSource is an inventory or event data source in a plugin, representing an executable to be invoked periodically to produce inventory data
type PluginSource struct {
	Command  []string          `yaml:"command"`  // Command to execute, run from the plugin's directory. The first element is the executable, subsequent elements are arguments.
	Prefix   ids.PluginID      `yaml:"prefix"`   // "Plugin path" for inventory data produced by the plugin. Not applicable for event sources.
	Interval int               `yaml:"interval"` // Number of seconds to wait between invocations of the source.
	Env      map[string]string `yaml:"env"`      // V1: Optional environment K/V pairs
}

// PluginProperty specifies a configuration parameter for a plugin
type PluginProperty struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

//==============================================================================
// Model for handling execution of plugins
//==============================================================================

// PluginInstance represents a running instance of a plugin with configured properties
type PluginInstance struct {
	plugin     *Plugin
	properties map[string]string
	sources    []*PluginSourceInstance
}

// PluginSourceInstance represents a running instance of a plugin data source within a plugin instance
type PluginSourceInstance struct {
	dataPrefix ids.PluginID // Property-substituted data prefix for this source instance
	command    []string     // Property-substituted command line for this source instance
	source     *PluginSource
}
