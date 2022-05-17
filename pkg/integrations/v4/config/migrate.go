// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Contains all the bits and pieces we need to parse and manage
// the external configuration

package config

import (
	"fmt"
	v3config "github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	metricArg      = "metrics"
	inventoryArg   = "inventory"
	eventsArg      = "events"
	exeSuffix      = ".exe"
	prefixArg      = "--"
	prefixArgShort = "-"
)

// V3toV4Result represents the result of a migration
type V3toV4Result struct {
	V3toV4Result string `json:"migrateV3toV4Result"`
}

func MigrateV3toV4(pathConfiguration string, pathDefinition string, pathOutput string, overwrite bool) error {

	if _, err := os.Stat(pathOutput); err == nil && !overwrite {
		return fmt.Errorf("file '%s' already exist and overwrite option is set to false", pathOutput)
	}

	// Reading old Definition file
	v3Definition := v3config.Plugin{}
	err := readAndUnmarshallConfig(pathDefinition, &v3Definition)
	if err != nil {
		return fmt.Errorf("error reading old config definition: %w", err)
	}

	// Reading old Configuration file
	v3Configuration := v3config.PluginInstanceWrapper{}
	err = readAndUnmarshallConfig(pathConfiguration, &v3Configuration)
	if err != nil {
		return fmt.Errorf("error reading old config configuration: %w", err)
	}

	// Populating new config
	v4config, err := populateV4Config(v3Definition, v3Configuration)
	if err != nil {
		return fmt.Errorf("error populating new config: %w", err)
	}

	// Writing output
	err = writeOutput(v4config, pathDefinition, pathConfiguration, pathOutput)
	if err != nil {
		return fmt.Errorf("error writing output: %w", err)
	}

	return nil
}

func readAndUnmarshallConfig(path string, out interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s, %w", path, err)
	}
	defer file.Close()

	err = yaml.NewDecoder(file).Decode(out)
	if err != nil {
		return fmt.Errorf("decoding %s, %w", path, err)
	}

	return nil
}

func populateV4Config(v3Definition v3config.Plugin, v3Configuration v3config.PluginInstanceWrapper) (*YAML, error) {
	if v3Configuration.IntegrationName != v3Definition.Name {
		return nil, fmt.Errorf("IntegrationName != Name: %s!=%s", v3Configuration.IntegrationName, v3Definition.Name)
	}

	// The field os does not have currently a simple way to be migrated
	if v3Definition.OS != "" {
		log.Debugf("The old definitions had a os directive, %s. Usually it is not needed, use `when` field otherwhise", v3Definition.OS)
	}

	v4Config := &YAML{}
	for commandName, pluginV1Command := range v3Definition.Commands {
		for _, pluginV1Instance := range v3Configuration.Instances {
			if commandName == pluginV1Instance.Command {
				integrationInstance := populateConfigEntry(pluginV1Instance, pluginV1Command)
				v4Config.Integrations = append(v4Config.Integrations, integrationInstance)
			}
		}
	}

	return v4Config, nil
}

func populateConfigEntry(pluginV1Instance *v3config.PluginV1Instance, pluginV1Command *v3config.PluginV1Command) ConfigEntry {
	configEntry := ConfigEntry{}
	if len(pluginV1Command.Command) == 0 {
		return configEntry
	}

	executable := pluginV1Command.Command[0]
	binaryName := filepath.Base(executable)
	configEntry.InstanceName = strings.TrimSuffix(binaryName, exeSuffix)
	configEntry.Interval = fmt.Sprintf("%ds", pluginV1Command.Interval)
	configEntry.Labels = pluginV1Instance.Labels
	configEntry.User = pluginV1Instance.IntegrationUser
	configEntry.InventorySource = pluginV1Command.Prefix.String()
	configEntry.Env = map[string]string{}
	for k, v := range pluginV1Instance.Arguments {
		configEntry.Env[strings.ToUpper(k)] = v
	}

	// Please notice that this is a simplification. If it is an absolute path we are adding it to the exec
	// if is a relative path or a integration name, we are assuming it is a standard integration included into the path
	if filepath.IsAbs(executable) {
		configEntry.Exec = pluginV1Command.Command
	} else {
		buildCLIArgs(pluginV1Command, &configEntry)
	}
	return configEntry
}

func buildCLIArgs(pluginV1Command *v3config.PluginV1Command, configEntry *ConfigEntry) {
	for index, arg := range pluginV1Command.Command {
		if index == 0 {
			// the first arg in command is the binary name
			continue
		}

		sanitized := strings.TrimPrefix(arg, prefixArg)
		sanitized = strings.TrimPrefix(sanitized, prefixArgShort)
		if sanitized == metricArg || sanitized == inventoryArg || sanitized == eventsArg {
			configEntry.Env[strings.ToUpper(sanitized)] = "true"
		} else {
			configEntry.CLIArgs = append(configEntry.CLIArgs, arg)
		}
	}
}

func writeOutput(v4Config *YAML, pathDefinition string, pathConfiguration string, pathOutput string) error {
	if v4Config == nil {
		return fmt.Errorf("v4Config pointer is nil")
	}

	file, err := os.OpenFile(pathOutput, os.O_RDWR|os.O_CREATE|syscall.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("opening File %s, %w", pathOutput, err)
	}
	defer file.Close()

	err = writeV4Config(v4Config, file)
	if err != nil {
		return fmt.Errorf("writing v4 config, %w", err)
	}

	err = writeFileAsComment(file, pathDefinition)
	if err != nil {
		return fmt.Errorf("adding old definition as comment, %w", err)
	}

	err = writeFileAsComment(file, pathConfiguration)
	if err != nil {
		return fmt.Errorf("adding old configuration as comment, %w", err)
	}

	return nil
}

func writeV4Config(v4Config *YAML, file *os.File) error {
	// see https://github.com/go-yaml/yaml/commit/7649d4548cb53a614db133b2a8ac1f31859dda8c
	//yaml.FutureLineWrap() @TODO enable this

	err := yaml.NewEncoder(file).Encode(*v4Config)
	if err != nil {
		return fmt.Errorf("writing v4ConfigBytes, %w", err)
	}

	return nil
}

func writeFileAsComment(file *os.File, filename string) error {
	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading file to add it as comment: %w", err)
	}

	fileCommented := strings.ReplaceAll(string(fileContent), "\n", "\n## ")
	_, err = file.Write([]byte("\n\n## " + fileCommented))
	if err != nil {
		return fmt.Errorf("writing text as comment: %w", err)
	}
	return nil
}
