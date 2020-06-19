// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	config2 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/when"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

const (
	minimumIntegrationInterval = config.FREQ_MINIMUM_EXTERNAL_PLUGIN_INTERVAL * time.Second
	defaultIntegrationInterval = config.FREQ_PLUGIN_EXTERNAL_PLUGINS * time.Second

	defaultTimeout = 120 * time.Second
	minimumTimeout = 100 * time.Millisecond
)

var ilog = log.WithComponent("integrations.Definition")

type IntegrationOutput struct {
	Output        executor.OutputReceive
	ExtraLabels   data.Map
	EntityRewrite []data.EntityRewrite
}

// InstancesLookup helps looking for integration executables that are not explicitly
// defined as an "exec" command
type InstancesLookup struct {
	// Legacy looks for integrations defined as legacy DefinitionCommands
	// allows referencing v3 integration definition files from v4 integration configurations
	Legacy func(DefinitionCommandConfig) (Definition, error)
	// ByName looks for the path of an executable only by the name of the integration
	ByName func(name string) (string, error)
}

// New interprets and validates a YAML ConfigEntry configuration and returns the proper
// implementation.
func New(te config2.ConfigEntry, lookup InstancesLookup, passthroughEnv []string, configTemplate []byte) (Definition, error) {

	if err := te.Sanitize(); err != nil {
		return Definition{}, err
	}
	d := Definition{
		ExecutorConfig: executor.Config{
			User:        te.User,
			Directory:   te.WorkDir,
			Environment: te.Env,
			Passthrough: passthroughEnv,
		},
		Labels:         te.Labels,
		Name:           te.Name,
		Interval:       getInterval(te.Interval),
		WhenConditions: conditions(te.When),
		ConfigTemplate: configTemplate,
		newTempFile:    newTempFile,
	}

	if te.InventorySource == "" {
		// Set to empty as currently Inventory source unknown
		d.InventorySource = ids.EmptyInventorySource
	} else {
		var err error
		d.InventorySource, err = ids.FromString(te.InventorySource)
		if err != nil {
			return Definition{}, errors.New("Error parsing 'inventory_source' YAML property: " + err.Error())
		}
	}

	// Unset timeout: default
	// Zero or negative: disabled
	if te.Timeout == nil {
		ilog.WithField("default_timeout", defaultTimeout).Debug("Setting default timeout.")
		d.Timeout = defaultTimeout
	} else if *te.Timeout <= 0 {
		ilog.WithField("timeout", *te.Timeout).Debug("Timeout disabled.")
		d.Timeout = 0
	} else if *te.Timeout < minimumTimeout {
		ilog.WithFields(logrus.Fields{
			"timeout":         te.Timeout,
			"minimum_timeout": minimumTimeout,
		}).Warn("timeout is too low (did you forget to append the time unit suffix?). Using minimum allowed value")
		d.Timeout = minimumTimeout
	} else {
		d.Timeout = *te.Timeout
	}

	// if looking for a v3 integration from the v4 engine
	if te.IntegrationName != "" {
		err := d.fromLegacyV3(te, lookup)
		return d, err
	}
	if te.Exec != nil {
		// if providing an executable path directly
		err := d.fromExecPath(te)
		return d, err
	}
	// if not an "exec" nor legacy integration, we'll look for an
	// executable corresponding to the "name" field in any of the integrations
	// folders, and wrap it into an "exec"
	err := d.fromName(te, lookup)
	return d, err
}

// LoadConfigTemplate loads the contents of an external configuration file that can be passed
// either as a path to the file in disk, or the contents embedded within a YAML
func LoadConfigTemplate(templatePath string, configContents interface{}) ([]byte, error) {
	if templatePath != "" {
		return ioutil.ReadFile(templatePath)
	}
	// if the integration config does not provide template nor template path, it's fine
	// we just don't do anything.
	if configContents == nil || configContents == "" {
		return nil, nil
	}
	// YAML 'config' section can be both a string or a YAML map (that will be converted to YAML text)
	switch cfg := configContents.(type) {
	case string:
		return []byte(cfg), nil
	case map[interface{}]interface{}, map[string]string, map[string]interface{}:
		var err error
		var template []byte
		if template, err = yaml.Marshal(cfg); err != nil {
			return nil, errors.New("can't convert 'config' YAML map into a string: " + err.Error())
		}
		return template, nil
	default:
		return nil, fmt.Errorf("'config' YAML property must be a string or a map. Found: %T", cfg)
	}
}

// loads the Definition Runnable from a reference to a Legacy V3 integration
func (d *Definition) fromLegacyV3(te config2.ConfigEntry, lookup InstancesLookup) error {
	var err error
	*d, err = lookup.Legacy(DefinitionCommandConfig{
		Common:          *d,
		IntegrationName: te.IntegrationName,
		Command:         te.Command,
		Arguments:       te.Arguments,
		DefaultPrefix:   d.InventorySource,
	})
	return err
}

// loads the Definition runnable from an executable path and arguments
func (d *Definition) fromExecPath(te config2.ConfigEntry) error {
	var err error
	d.runnable, err = newExecutor(&d.ExecutorConfig, te.Exec)
	return err
}

// loads the Definition runnable from an executable name, looking for it into a
// set of predefined folders
func (d *Definition) fromName(te config2.ConfigEntry, lookup InstancesLookup) error {
	// if not an "exec" nor legacy integration, we'll look for an
	// executable corresponding to the "name" field in any of the integrations
	// folders, and wrap it into an "exec"
	path, err := lookup.ByName(te.Name)
	if err != nil {
		return errors.New("can't instantiate integration: " + err.Error())
	}
	// we need to pass the path as part of an array, to avoid splitting the
	// folders as different arguments
	te.Exec = []string{path}
	d.runnable, err = newExecutor(&d.ExecutorConfig, te.Exec)
	return err
}

// getInterval returns a task interval according to a given set of limitations and policies:
// - If no duration string is provided, it returns the default interval
// - If a wrong string is provided, it returns the default interval and logs a warning message
// - If the provided integration is lower than the minimum allowed, it logs a warning message and returns the minimum
func getInterval(duration string) time.Duration {
	if duration == "" {
		return defaultIntegrationInterval
	}
	d, err := time.ParseDuration(duration)
	if err != nil {
		ilog.WithError(err).WithFields(logrus.Fields{
			"interval": duration,
			"default":  defaultIntegrationInterval.String(),
		}).Warn("invalid integration interval. Using default")
		return defaultIntegrationInterval
	}
	if d < minimumIntegrationInterval {
		ilog.WithError(err).WithFields(logrus.Fields{
			"interval": duration,
			"minimum":  minimumIntegrationInterval.String(),
		}).Warn("integration interval is lower than the minimum allowed. Using the minimum interval")
		return minimumIntegrationInterval
	}
	return d
}

// get condition functions from the YAML 'when:' section
func conditions(enabling config2.EnableConditions) []when.Condition {
	var conds []when.Condition

	// We do not consider here FeatureFlag as it is managed at the integrations manager
	if enabling.FileExists != "" {
		conds = append(conds, when.FileExists(enabling.FileExists))
	}

	if len(enabling.EnvExists) > 0 {
		conds = append(conds, when.EnvExists(enabling.EnvExists))
	}
	return conds
}
