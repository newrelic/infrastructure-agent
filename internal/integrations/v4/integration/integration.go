// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/when"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	config2 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

const (
	defaultIntegrationInterval = config.FREQ_PLUGIN_EXTERNAL_PLUGINS * time.Second
	defaultTimeout             = 120 * time.Second
	minimumTimeout             = 100 * time.Millisecond
	intervalEnvVarName         = "NRI_CONFIG_INTERVAL"
)

var minimumIntegrationIntervalOverride = ""

var minimumIntegrationInterval = func() time.Duration {
	if minimumIntegrationIntervalOverride != "" {
		v, err := time.ParseDuration(minimumIntegrationIntervalOverride)
		if err == nil {
			return v
		}
	}
	return config.FREQ_MINIMUM_EXTERNAL_PLUGIN_INTERVAL * time.Second
}()

var ilog = log.WithComponent("integrations.Definition")

type Output struct {
	Receive       executor.OutputReceive
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

func newDefinitionWithoutLookup(ce config2.ConfigEntry, passthroughEnv []string, configTemplate []byte) (Definition, error) {

	if err := ce.Sanitize(); err != nil {
		return Definition{}, err
	}

	ce.UppercaseEnvVars()

	interval := getInterval(ce.Interval)
	// Reading this env the integration can know configured interval.
	ce.Env[intervalEnvVarName] = fmt.Sprintf("%v", interval)

	d := Definition{
		ExecutorConfig: executor.Config{
			User:            ce.User,
			Directory:       ce.WorkDir,
			IntegrationName: ce.InstanceName,
			Environment:     ce.Env,
			Passthrough:     passthroughEnv,
		},
		Labels:         ce.Labels,
		Tags:           ce.Tags,
		Name:           ce.InstanceName,
		Interval:       interval,
		LogsQueueSize:  ce.LogsQueueSize,
		WhenConditions: conditions(ce.When),
		ConfigTemplate: configTemplate,
		newTempFile:    newTempFile,
	}

	if ce.InventorySource == "" {
		// Set to empty as currently Inventory source unknown
		d.InventorySource = ids.EmptyInventorySource
	} else {
		var err error
		d.InventorySource, err = ids.FromString(ce.InventorySource)
		if err != nil {
			return Definition{}, errors.New("Error parsing 'inventory_source' YAML property: " + err.Error())
		}
	}

	// Unset timeout: default
	// Zero or negative: disabled
	if ce.HeartbeatTimeout == "" {
		ilog.WithField("default_timeout", defaultTimeout).Debug("Setting default timeout.")
		d.Timeout = defaultTimeout
	}
	duration, err := time.ParseDuration(ce.HeartbeatTimeout)
	if err != nil {
		ilog.WithError(err).WithFields(logrus.Fields{
			"timeout": duration,
			"default": defaultTimeout.String(),
		}).Warn("invalid integration timeout. Using default")
		d.Timeout = defaultTimeout
	} else if duration <= 0 {
		ilog.WithField("timeout", duration).Debug("Timeout disabled.")
		d.Timeout = 0
	} else if duration < minimumTimeout {
		ilog.WithFields(logrus.Fields{
			"timeout":         ce.Timeout,
			"minimum_timeout": minimumTimeout,
		}).Warn("timeout is too low (did you forget to append the time unit suffix?). Using minimum allowed value")
		d.Timeout = minimumTimeout
	} else {
		d.Timeout = duration
	}

	return d, nil
}

// NewDefinition creates Definition from ConfigEntry, config template, executables lookup and
// passed through env vars.
func NewDefinition(ce config2.ConfigEntry, lookup InstancesLookup, passthroughEnv []string, configTemplate []byte) (d Definition, err error) {
	d, err = newDefinitionWithoutLookup(ce, passthroughEnv, configTemplate)
	if err != nil {
		return
	}

	// if looking for a v3 integration from the v4 engine
	if ce.IntegrationName != "" {
		err = d.fromLegacyV3(ce, lookup)
		return
	}
	if ce.Exec != nil {
		// if providing an executable path directly
		err = d.fromExecPath(ce)
		return
	}
	// if not an "exec" nor legacy integration, we'll look for an
	// executable corresponding to the "name" field in any of the integrations
	// folders, and wrap it into an "exec"
	err = d.fromName(ce, lookup)
	return
}

// NewAPIDefinition creates a definition generated for payload coming from API.
func NewAPIDefinition(integrationName string) (d Definition, err error) {
	ce := config2.ConfigEntry{
		InstanceName: integrationName,
	}
	d, err = newDefinitionWithoutLookup(ce, nil, nil)

	return
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
	path, err := lookup.ByName(te.InstanceName)
	if err != nil {
		return errors.New("can't instantiate integration: " + err.Error())
	}
	// we need to pass the path as part of an array, to avoid splitting the
	// folders as different arguments
	te.Exec = append([]string{path}, te.CLIArgs...)
	d.runnable, err = newExecutor(&d.ExecutorConfig, te.Exec)
	return err
}

// getInterval returns a task interval according to a given set of limitations and policies:
// - If no duration string is provided, it returns the default interval
// - If a wrong string is provided, it returns the default interval and logs a warning message
// - If the provided integration is lower than the minimum allowed, it logs a warning message and returns the minimum
func getInterval(duration string) time.Duration {
	// zero value disables interval
	if duration == "0" {
		return 0
	}

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

// ErrLookup is a test helper that returns errors.
var ErrLookup = InstancesLookup{
	Legacy: func(_ DefinitionCommandConfig) (Definition, error) {
		return Definition{}, errors.New("legacy integrations provider not expected to be invoked")
	},
	ByName: func(_ string) (string, error) {
		return "", errors.New("lookup by name not expected to be invoked")
	},
}
