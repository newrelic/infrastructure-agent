// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/when"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track/ctx"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"io/ioutil"
	"os"
	"strings"
)

const (
	configPathEnv     = "CONFIG_PATH"
	configPathVarName = "config.path"
	configPathHolder  = "${" + configPathVarName + "}"
)

var elog = log.WithComponent("integrations.Definition")

// Definition is a n `-exec` yaml entry. It will execute the provided command line or array of commands
type Definition struct {
	Name            string
	Labels          map[string]string
	ExecutorConfig  executor.Config
	Interval        time.Duration
	Timeout         time.Duration
	ConfigTemplate  []byte // external configuration file, if provided
	InventorySource ids.PluginID
	WhenConditions  []when.Condition
	CmdChanReq      *ctx.CmdChannelRequest // not empty: command-channel run/stop integration requests
	runnable        executor.Executor
	newTempFile     func(template []byte) (string, error)
}

func (d *Definition) TimeoutEnabled() bool {
	return d.Timeout > 0
}

func (d *Definition) SingleRun() bool {
	return d.Interval == 0
}

// PluginID returns inventory plugin ID
func (d *Definition) PluginID(integrationName string) ids.PluginID {
	// user specified an inventory source has precedence
	if d.InventorySource != ids.EmptyInventorySource {
		return d.InventorySource
	}
	// else possible plugin returned name
	if integrationName != "" {
		return ids.NewDefaultInventoryPluginID(integrationName)
	}

	// fallback to plugin name from config
	return ids.NewDefaultInventoryPluginID(d.Name)
}

func (d *Definition) Run(ctx context.Context, bindVals *databind.Values, pidC, exitCodeC chan<- int) ([]Output, error) {
	logger := elog.WithField("integration_name", d.Name)
	logger.Debug("Running task.")
	// no discovery data: execute a single instance
	if bindVals == nil {
		logger.Debug("Running single instance.")
		return []Output{{Receive: d.runnable.Execute(ctx, pidC, exitCodeC)}}, nil
	}

	// apply discovered data to run multiple instances
	var tasksOutput []Output

	// merges both runnable configuration and config template (if any) to avoid having different
	// discoverable
	type discoveredConfig struct {
		Executor       executor.Executor
		ConfigTemplate []byte
	}

	// used to post-process "${config.path}" appearances only if we have found it previously
	foundConfigPath := false
	onDemand := noOnDemand
	if d.ConfigTemplate != nil {
		onDemand = ignoreConfigPathVar(&foundConfigPath)
	}
	matches, err := databind.Replace(bindVals, discoveredConfig{
		Executor:       d.runnable.DeepClone(),
		ConfigTemplate: d.ConfigTemplate,
	}, databind.Provided(onDemand))
	if err != nil {
		return nil, err
	}

	logger.Debug("Running through all discovery matches.")
	for _, ir := range matches {
		dc, ok := ir.Variables.(discoveredConfig)
		if !ok { // should never happen, but left here for type safety
			elog.WithField("type", fmt.Sprintf("%T", ir)).
				Warn("can't execute integration due to an unexpected Executor type")
			continue
		}

		var removeFile func(<-chan struct{})
		if dc.ConfigTemplate != nil {
			templateFile, err := d.newTempFile(dc.ConfigTemplate)
			if err != nil {
				return nil, err
			}
			// Setting to remove this file after the integration has finished
			removeFile = removeTempFile(templateFile)

			// If we previously detected some "${config.path}" placeholder in the arguments
			// or the environment, we look again for it and replace it by the
			// template file. Otherwise, we set the default "CONFIG_PATH"
			// environment variable
			if foundConfigPath {
				// replacing on the environment
				for key, value := range dc.Executor.Cfg.Environment {
					if strings.Contains(value, configPathHolder) {
						dc.Executor.Cfg.Environment[key] = strings.Replace(value, configPathHolder, templateFile, -1)
					}
				}
				// replacing on the command line arguments
				for i, value := range dc.Executor.Args {
					if strings.Contains(value, configPathHolder) {
						dc.Executor.Args[i] = strings.Replace(value, configPathHolder, templateFile, -1)
					}
				}
			} else {
				dc.Executor.Cfg.Environment[configPathEnv] = templateFile
			}
		} else {
			logger.Debug("Found a nil ConfigTemplate.")
		}

		logger.Debug("Executing task.")
		taskOutput := dc.Executor.Execute(ctx, nil, nil)
		if removeFile != nil {
			go removeFile(taskOutput.Done)
		}
		tasksOutput = append(tasksOutput, Output{Receive: taskOutput, ExtraLabels: ir.MetricAnnotations, EntityRewrite: ir.EntityRewrites})
	}
	return tasksOutput, nil
}

// remoteTempFile returns a function that removes the file corresponding to the passed path when the provided channel
// is closed
func removeTempFile(path string) func(<-chan struct{}) {
	return func(done <-chan struct{}) {
		<-done
		if err := os.Remove(path); err != nil {
			elog.WithError(err).WithField("path", path).Warn("can't remove temporary integration config file")
		}
	}
}

// For configuration databind.Replace,
// dynamic databinding provider that just ignores the ${config.path} variable and leaves
// it there to avoid a "Variable Not Found", as this variables has not been discovered
// nor bound from any secret variable). We keep it unchanged for its later replacement.
func ignoreConfigPathVar(foundVar *bool) databind.OnDemand {
	return func(key string) (value []byte, found bool) {
		if key == configPathVarName {
			*foundVar = true
			return []byte(configPathHolder), true
		}
		return nil, false
	}
}

// For configuration databind.Replace, just ignoring any variable that has not been already discovered
func noOnDemand(_ string) ([]byte, bool) {
	return nil, false
}

// returns the file name
func newTempFile(template []byte) (string, error) {
	// create it
	file, err := ioutil.TempFile("", "discovered")
	if err != nil {
		return "", errors.New("can't create config file template: " + err.Error())
	}
	defer func() { _ = file.Close() }()
	elog.WithField("file", file.Name()).Debug("Creating discovered file.")
	if _, err := file.Write(template); err != nil {
		return "", errors.New("can't write into config file template: " + err.Error())
	}
	return file.Name(), nil
}

// returns an executor from the given execPath, that can be a string (one-line path and arguments),
// or an array (first element is the executable path and the rest are the arguments)
func newExecutor(executorConfig *executor.Config, execPath config.ShlexOpt) (executor.Executor, error) {
	if len(execPath) == 0 {
		return executor.Executor{}, errors.New("exec command can't be empty")
	}
	return executor.FromCmdSlice(execPath, executorConfig), nil
}
