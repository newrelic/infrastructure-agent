// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v3legacy

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"

	"gopkg.in/yaml.v2"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/files"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

var drlog = log.WithComponent("integrations.DefinitionsRepo")

// Definition reads the data from a legacy *-definition.yml integration file.
// For integrations v4, this struct replaces the externalPlugins.Plugin struct and
// omits fields that are unnecessary or undocumented.
type Definition struct {
	Name     string             `yaml:"name"`
	Commands map[string]Command `yaml:"commands"`
}

// Command stores the information of a defined command
type Command struct {
	Command  []string `yaml:"command"`
	Interval int      `yaml:"interval"`
	Prefix   string   `yaml:"prefix"`
}

// DefinitionContext stores contextual information about a Definition file,
// such as the directory where it is defined (used then to look for executables and files
// when the integration is run).
type DefinitionContext struct {
	Dir        string
	Definition Definition
}

type LegacyConfig struct {
	DefinitionFolders []string
	Verbose           int
}

// DefinitionsRepo stores all the definitions from all the files
type DefinitionsRepo struct {
	Config LegacyConfig
	// Key: integration name
	Definitions map[string]DefinitionContext
}

// NewDefinitionsRepo returns a DefinitionsRepo containing all the integrations and
// commands from the passed folders
func NewDefinitionsRepo(cfg LegacyConfig) DefinitionsRepo {
	dr := DefinitionsRepo{Config: cfg, Definitions: map[string]DefinitionContext{}}
	for _, folder := range cfg.DefinitionFolders {
		dr.loadDefinitions(folder)
	}
	return dr
}

// loads all the definitions from the given folder and stores them into the Repo
func (dr *DefinitionsRepo) loadDefinitions(folder string) {
	yamlFiles, err := files.AllYAMLs(folder)
	flog := drlog.WithField("folder", folder)
	if err != nil {
		if os.IsNotExist(err) {
			flog.Debug("Folder does not exist. Ignoring.")
		} else {
			flog.WithError(err).Warn("can't look for integrations in the folder. Ignoring")
		}
		return
	}
	for _, file := range yamlFiles {
		fflog := flog.WithField("file", file.Name())
		contents, err := ioutil.ReadFile(filepath.Join(folder, file.Name()))
		if err != nil {
			fflog.WithError(err).Warn("can't read file. Ignoring")
			continue
		}
		var def Definition
		if err := yaml.Unmarshal(contents, &def); err != nil {
			fflog.WithError(err).Warn("invalid YAML file. Ignoring")
			continue
		}
		if def.Name == "" {
			fflog.Warn("definition file does not contain an integration name. Ignoring")
			continue
		}
		// Once the definition file is memory and validated, we store all the integrations we
		// find into the definitions repo
		dr.Definitions[def.Name] = DefinitionContext{
			Dir:        folder,
			Definition: def,
		}
	}
}

// NewDefinitionCommand uses the command reference and arguments from the integrations' entry to
// look for a v3 integration in the command repository, and returning the corresponding Integration implementation
func (dcb *DefinitionsRepo) NewDefinitionCommand(dcc integration.DefinitionCommandConfig) (integration.Definition, error) {
	definition, ok := dcb.Definitions[dcc.IntegrationName]
	if !ok {
		return integration.Definition{}, errors.New("integration definition not found: " + dcc.IntegrationName)
	}
	command, ok := definition.Definition.Commands[dcc.Command]
	if !ok {
		return integration.Definition{}, errors.New("integration " + dcc.IntegrationName + " does not have any command named " + dcc.Command)
	}

	// If the interval is defined both in the config and the definition,
	// we'll prioritize the config one
	if dcc.Common.Interval == 0 {
		dcc.Common.Interval = time.Duration(command.Interval) * time.Second
	}

	// Legacy integrations don't allow setting the environment, as it will be used
	// for passing the "arguments" entry.
	dcc.Common.ExecutorConfig.Environment =
		legacy.ArgumentsToEnvVars(dcb.Config.Verbose, dcc.Arguments)

	// We need to set the Working Directory to the folder where the definition file is placed
	dcc.Common.ExecutorConfig.Directory = definition.Dir

	if command.Prefix == "" {
		dcc.Common.InventorySource = dcc.DefaultPrefix
	} else {
		var err error
		dcc.Common.InventorySource, err = ids.FromString(command.Prefix)
		if err != nil {
			return integration.Definition{}, errors.New("invalid 'prefix' value in integration definition: " + err.Error())
		}
	}

	return integration.FromLegacyDefinition(dcc,
		executor.FromCmdSlice(
			command.Command, &dcc.Common.ExecutorConfig),
	), nil
}
