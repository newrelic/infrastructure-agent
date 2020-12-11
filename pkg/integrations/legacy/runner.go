// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/pkg/errors"

	"github.com/golang/groupcache/lru"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"
)

const (
	maxEntityAttributeCount    = 240 // 254 - 14 (reserved for agent decorations) https://docs.newrelic.com/docs/insights/insights-data-sources/custom-data/insights-custom-data-requirements-limits
	entityMetricsLengthWarnMgs = "metric attributes exceeds 240 limit, some might be lost"

	// These two constants can be found in V4 integrations as well
	labelPrefix     = "label."
	labelPrefixTrim = 6
)

var (
	DefaultInheritedEnv = []string{"PATH"}
	// finds matches of either ${blahblah} or $blahblha (and groups them)
	regex, _ = regexp.Compile(`\$\{(.+?)[}]|\$(.+)`)
	rlog     = log.WithComponent("PluginRunner")
	logLRU   = lru.New(1000) // avoid flooding the log with violations for the same entity
)

type PluginRunner struct {
	instances []*PluginInstance
	registry  *PluginRegistry
	closeWait *sync.WaitGroup
	agent     iAgent
}

type iAgent interface {
	RegisterPlugin(agent.Plugin)
	GetContext() agent.AgentContext
}

func NewPluginRunner(registry *PluginRegistry, agent iAgent) *PluginRunner {
	return &PluginRunner{
		registry:  registry,
		closeWait: &sync.WaitGroup{},
		agent:     agent,
	}
}

func newExternalV1Plugin(
	runner *PluginRunner,
	instance *PluginV1Instance) (*externalPlugin, error) {

	command, ok := instance.plugin.Commands[instance.Command]
	if !ok {
		return nil, fmt.Errorf(
			"couldn't find the command '%s' for the integration instance '%s'",
			instance.Command,
			instance.Name,
		)
	}

	ctx := runner.agent.GetContext()

	p := &externalPlugin{
		PluginCommon: agent.NewExternalPluginCommon(
			command.Prefix,
			ctx,
			instance.plugin.Name,
		),
		pluginRunner:   runner,
		pluginInstance: instance,
		pluginCommand:  command,
		activeEntities: ctx.ActiveEntitiesChannel(),
		healthCheck:    true,
		workingDir:     runner.registry.GetPluginDir(instance.plugin),
		binder:         databind.New(),
		lock:           &sync.RWMutex{},
	}
	p.PluginCommon.DetailedLogFields = p.detailedLogFields
	p.PluginCommon.LogFields = p.defaultLogFields()
	p.logger = p.newLogger()
	return p, nil
}

func (pr *PluginRunner) ConfigureV1Plugins(_ agent.AgentContext) (err error) {
	instances := pr.registry.GetPluginInstances()
	for _, instance := range instances {
		pr.closeWait.Add(1)
		plugin, err := newExternalV1Plugin(pr, instance)
		if err != nil {
			rlog.WithIntegration(instance.plugin.Name).WithError(err).Error("configuring V1 plugin")
			continue
		}
		pr.agent.RegisterPlugin(plugin)
	}
	return
}

// ConfigurePlugin sets up the plugin to run all data sources as configured. This function will create numerous goroutines responsible for running the data sources on schedule.
func (pr *PluginRunner) ConfigurePlugin(pluginConfig PluginConfig, activeEntitiesChannel chan string) (err error) {
	plugin, err := pr.registry.GetPlugin(pluginConfig.PluginName)
	if err != nil {
		return err
	}

	// Instantiate plugin instances and do some basic checking of properties and instance counts
	var pluginInstances []*PluginInstance
	if len(plugin.Properties) > 0 {
		// If a plugin specifies properties, we must have at least one configured instance of that plugin.
		if len(pluginConfig.PluginInstances) == 0 {
			return fmt.Errorf("no integration instances configured for %v, but the integration specifies properties which must be configured per-instance. Please specify at least one instance configuration for this integration before running it", plugin.Name)
		}
		// Create a plugin instance for each set of properties configured for this plugin
		for _, instanceProperties := range pluginConfig.PluginInstances {
			pluginInstances = append(pluginInstances, newPluginInstance(plugin, instanceProperties))
		}
	} else {
		// No properties defined, so this is a simple plugin of which we can only have one instance.
		if len(pluginConfig.PluginInstances) > 0 {
			return fmt.Errorf("one or more integration instances are configured for %v, but the integration specifies no properties and so cannot have instance configuration. Please remove all instances of this integration before running it", plugin.Name)
		}
		// Configuration-free plugins get a default instance with no properties
		pluginInstances = append(pluginInstances, newPluginInstance(plugin, make(map[string]string)))
	}

	if err = pr.checkPrefixDuplicates(pluginInstances); err != nil {
		return
	}

	pr.registerInstances(pluginInstances, activeEntitiesChannel)
	pr.instances = append(pr.instances, pluginInstances...)

	rlog.WithIntegration(plugin.Name).WithField("instances", len(pluginInstances)).
		Debug("Integration started.")

	return
}

func (pr *PluginRunner) Wait() {
	pr.closeWait.Wait()
}

func newPluginInstance(plugin *Plugin, properties map[string]string) *PluginInstance {
	addStandardProperties(properties)

	pathSafeProperties := make(map[string]string)
	for key, value := range properties {
		pathSafeProperties[key] = strings.Replace(value, "/", "_", -1)
	}

	sources := make([]*PluginSourceInstance, len(plugin.Sources))
	for i, source := range plugin.Sources {
		dataPrefix := ids.PluginID{
			Category: substituteProperties(source.Prefix.Category, pathSafeProperties),
			Term:     substituteProperties(source.Prefix.Term, pathSafeProperties),
		}
		command := make([]string, len(source.Command))
		for i, commandPart := range source.Command {
			command[i] = substituteProperties(commandPart, properties)
		}
		sources[i] = &PluginSourceInstance{
			source:     source,
			dataPrefix: dataPrefix,
			command:    command,
		}
	}

	return &PluginInstance{
		plugin:     plugin,
		properties: properties,
		sources:    sources,
	}
}

func addStandardProperties(properties map[string]string) {
	properties["os"] = runtime.GOOS
	properties["arch"] = runtime.GOARCH
}

func substituteProperties(value string, properties map[string]string) string {
	// Make a reverse-alphabetical list of property names. Reverse-alphabetical ensures that we'll
	// handle longer property names before their prefixes, so if I have a "foo" and "fooBar" property,
	// "$fooBar" should always end up being replaced with fooBar's value, not treated as ($foo)Bar
	propNames := make([]string, len(properties))
	i := 0
	for propName := range properties {
		propNames[i] = propName
		i++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(propNames)))

	for _, propName := range propNames {
		if propName != "" {
			value = strings.Replace(value, fmt.Sprintf("$%s", propName), properties[propName], -1)
		}
	}

	return value
}

// checkPrefixDuplicates returns an error if there are any duplicated data prefixes either
// in the given set of plugin instances or between them and all previously registered plugin instances.
func (pr *PluginRunner) checkPrefixDuplicates(instances []*PluginInstance) error {
	thisPluginPrefixes := make(map[ids.PluginID]bool)
	for _, instance := range instances {
		for _, source := range instance.sources {
			if thisPluginPrefixes[source.dataPrefix] {
				return fmt.Errorf("Multiple data sources with a prefix of %s were found for the %s integration. Please ensure there are no duplicate configurations for the integration.", source.dataPrefix, instance.plugin.Name)
			}
			thisPluginPrefixes[source.dataPrefix] = true
		}
	}

	for _, instance := range pr.instances {
		for _, source := range instance.sources {
			if thisPluginPrefixes[source.dataPrefix] {
				return fmt.Errorf("Multiple integration (%s and %s) were found with data source prefix %s. Only one data source can be configured for the same data source prefix.", instances[0].plugin.Name, instance.plugin.Name, source.dataPrefix)
			}
		}
	}

	return nil
}

// startInstances starts a goroutine to execute the plugin command for each data source in the given plugin instance.
func (pr *PluginRunner) registerInstances(instances []*PluginInstance, activeEntities chan string) {
	for _, instance := range instances {
		for _, source := range instance.sources {
			pr.closeWait.Add(1)
			plugin := externalPlugin{
				PluginCommon: agent.NewExternalPluginCommon(
					source.dataPrefix,
					pr.agent.GetContext(),
					instance.plugin.Name,
				),
				pluginRunner: pr,
				pluginInstance: &PluginV1Instance{
					Name:      instance.plugin.Name,
					Arguments: source.source.Env,
					plugin:    instance.plugin,
				},
				pluginCommand: &PluginV1Command{
					Command:  source.command,
					Prefix:   source.dataPrefix,
					Interval: source.source.Interval,
				},
				activeEntities: activeEntities,
				healthCheck:    true,
				workingDir:     pr.registry.GetPluginDir(instance.plugin),
				binder:         databind.New(),
			}
			plugin.PluginCommon.DetailedLogFields = plugin.detailedLogFields
			plugin.PluginCommon.LogFields = plugin.defaultLogFields()
			plugin.logger = plugin.newLogger()

			pr.agent.RegisterPlugin(&plugin)
		}
	}
}

type externalPlugin struct {
	agent.PluginCommon
	lock           *sync.RWMutex
	activeEntities chan string // Reports back active entities
	healthCheck    bool
	pluginRunner   *PluginRunner
	pluginInstance *PluginV1Instance
	pluginCommand  *PluginV1Command
	cmdWrappers    []*cmdWrapper // one command per discovery match
	cachePath      string
	logger         log.Entry
	workingDir     string
	binder         databind.Binder
}

func (ep *externalPlugin) getCmdWrappers() []*cmdWrapper {
	ep.lock.RLock()
	defer ep.lock.RUnlock()
	cmds := make([]*cmdWrapper, len(ep.cmdWrappers))
	copy(cmds, ep.cmdWrappers)
	return cmds
}

func (ep *externalPlugin) newLogger() log.Entry {
	return log.WithFieldsF(ep.defaultLogFields)
}

// Kill kills all the processes that may be running for this plugin
func (ep *externalPlugin) Kill() {
	for _, cmd := range ep.getCmdWrappers() {
		if cmd.cmd.Process != nil {
			llog := rlog.WithIntegration(ep.pluginInstance.Name).WithField("pid", cmd.cmd.Process.Pid)
			llog.Info("killing process")
			if err := cmd.cmd.Process.Kill(); err != nil {
				llog.WithError(err).Warn("can't kill process")
			}
		}
	}
}

func (ep *externalPlugin) name() string {
	return ep.pluginInstance.Name
}

func (ep *externalPlugin) prefix() ids.PluginID {
	return ep.pluginCommand.Prefix
}

// defaultlogFields returns the default logrus `logrus.Fields` that should be
// used for the external plugin logs.
func (ep *externalPlugin) defaultLogFields() logrus.Fields {
	return logrus.Fields{
		"working-dir": ep.workingDir,
		"prefix":      ep.prefix(),
		"integration": ep.pluginInstance.plugin.Name,
		"instance":    ep.name(),
	}
}

// detailedLogFields returns the complete information of the external
// plugin, including the environment variables. Environment variables
// are retrieved in each function execution, as it is when executing the
// integration commands.
//
// This fields are used primarily by the agent when requesting the
// information for each external plugin.
func (ep *externalPlugin) detailedLogFields() logrus.Fields {
	return logrus.Fields{
		"workingDir":      ep.workingDir,
		"prefix":          ep.prefix(),
		"integration":     ep.pluginInstance.plugin.Name,
		"instance":        ep.name(),
		"os":              ep.pluginInstance.plugin.OS,
		"protocolVersion": ep.pluginInstance.plugin.ProtocolVersion,
		"labels":          ep.pluginInstance.Labels,
		"interval":        ep.pluginCommand.Interval,
		"arguments":       helpers.ObfuscateSensitiveDataFromMap(ep.pluginInstance.Arguments),
		"command":         ep.pluginInstance.Command,
		"commandLine":     helpers.ObfuscateSensitiveDataFromArray(ep.pluginCommand.Command),
		"env-vars":        helpers.ObfuscateSensitiveDataFromMap(ep.envVars()),
	}
}

// ArgumentsToEnvVars returns the environment variables that will be passed to the
// external plugin command. This implies that the plugin arguments are
// passed as environment variables to the integrations.
func ArgumentsToEnvVars(verbose int, arguments map[string]string) map[string]string {
	envVars := make(map[string]string)
	envVars["VERBOSE"] = fmt.Sprintf("%v", verbose)

	// Pass the integration arguments as environment variables to the command
	for k, v := range arguments {
		envVars[strings.ToUpper(k)] = expand(v)
	}
	return envVars
}

func expand(v string) string {
	matches := regex.FindAllStringSubmatch(v, -1)
	// if we have matches, the string we want is either in index 1 or 2 of the match
	for _, ms := range matches {
		if len(ms) > 0 {
			match := ms[1]
			if len(match) <= 0 {
				match = ms[2]
			}
			// if we have an env var value for the match we replace it in the original string
			if val, found := os.LookupEnv(match); found {
				v = strings.Replace(v, ms[0], val, 1)
			}
		}
	}
	// no match, return as-is
	return v
}

// Appends the passthrough environment to the calculated environment variables
func (ep *externalPlugin) appendEnvPassthrough(envVars map[string]string) {
	// Pass the global environment variables defined in configuration, plus any defaults we always pass through
	dedupedPassthrough := make(map[string]bool)
	for _, k := range append(ep.Context.Config().PassthroughEnvironment, DefaultInheritedEnv...) {
		dedupedPassthrough[k] = true
	}
	for k := range dedupedPassthrough {
		if os.Getenv(k) != "" {
			// Warn the user if we're stepping on something else they set
			if envVars[k] != "" {
				ep.logger.WithField("envVar", k).
					Warn("Environment variable is passed through to all integrations, but this integration " +
						"uses the same name for a configuration option. The environment variable value will " +
						"replace the value set in the integration configuration.")
			}
			envVars[k] = os.Getenv(k)
		}
	}
}

// The environment is constructed from the env
// vars specified in the `PassthroughEnvironment`, the `DefaultInheritedEnv`
// and the plugin arguments;
// In the case that a value is defined in both the plugin arguments and the
// `PassthroughEnvironment`, the value from the environment takes precedence.
func (ep *externalPlugin) envVars() map[string]string {
	cfg := ep.Context.Config()
	envVars := ArgumentsToEnvVars(cfg.Verbose, ep.pluginInstance.Arguments)
	ep.appendEnvPassthrough(envVars)
	return envVars
}

func (ep *externalPlugin) logIfHealthCheck(message string) {
	if ep.healthCheck {
		ep.logger.Info(message)
	}
}

// startInstance is the main worker loop for each data source in a plugin.
func (ep *externalPlugin) Run() {
	defer func() {
		ep.pluginRunner.closeWait.Done()
		if err := recover(); err != nil {
			ep.logger.WithField("error", err).Debug("Stopping integration data source: fatal error gathering data.")
		}
	}()

	command := ep.pluginCommand
	first := true
	interval := config.ValidateConfigFrequencySetting(
		int64(command.Interval),
		config.FREQ_MINIMUM_EXTERNAL_PLUGIN_INTERVAL,
		config.FREQ_PLUGIN_EXTERNAL_PLUGINS,
		false,
	)

	if interval <= config.FREQ_DISABLE_SAMPLING {
		ep.logger.Debug("Disabled.")
		return
	}

	if interval > time.Duration(config.FREQ_MAXIMUM_EXTERNAL_PLUGIN_INTERVAL) {
		ep.logger.WithField("intervalSecs", interval).Warn("Interval value might not work with the alerts system.")
	}

	// Wait for a random amount of time the first time we run this plugin. This should help to spread the
	// plugins apart so we don't end up running tons of things at the same instant.
	t := time.Second * time.Duration(rand.Int63n(int64(interval)))

	for {
		ep.logIfHealthCheck("Integration health check starting")
		pluginDir := ep.workingDir

		// Runs all the commands that may have been created after
		// the discovery/databind process is applied to the plugin instance
		// (if no discovery is triggered, it will be 1 command)
		ep.updateCmdWrappers(pluginDir)
		for _, cmd := range ep.getCmdWrappers() {
			ep.runCmd(cmd, t, first, interval)
		}
		select {
		case <-ep.HealthCheckCh:
			ep.healthCheck = true
		case <-time.After(t):
			if first {
				first = false
				t = time.Second * interval
			}
			if ep.healthCheck {
				ep.healthCheck = false
			}
		}
	}
}

func (ep *externalPlugin) runCmd(cmd *cmdWrapper, _ time.Duration, _ bool, _ time.Duration) {
	// Use fields inside a function to avoid obfuscation overhead
	ep.logger.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"path": cmd.cmd.Path,
			"args": helpers.ObfuscateSensitiveDataFromArray(cmd.cmd.Args),
			"env":  helpers.ObfuscateSensitiveDataFromArray(cmd.cmd.Env),
		}
	}).Debug("Running command.")
	cmdOut, err := cmd.cmd.StdoutPipe()
	if err != nil {
		ep.logger.WithError(err).Error("getting output pipe")
	}
	cmdErr, err := cmd.cmd.StderrPipe()
	if err != nil {
		ep.logger.WithError(err).Error("getting error pipe")
	}
	if err := cmd.cmd.Start(); err != nil {
		ep.logger.WithError(err).Error("starting data source")
	}
	payloadStatusCh := make(chan bool)
	go func() {
		ok := ep.handleOutput(cmdOut, cmd.metricAnnotations, cmd.entityRewrite)
		payloadStatusCh <- ok
	}()
	stderrChan := make(chan []byte)
	go func() {
		c, err := ioutil.ReadAll(cmdErr)
		if err != nil {
			ep.logger.WithError(err).Error("reading stderr")
		}

		stderrChan <- c
	}()
	errText := <-stderrChan
	payloadOk := <-payloadStatusCh
	commandOk := true
	// and overrides per plugin in their own configs.
	if err := cmd.cmd.Wait(); err != nil {
		ep.logger.WithField("stderr", string(errText)).
			WithError(err).Error("Integration command failed")
		commandOk = false
	} else {
		if len(errText) > 0 {
			ep.logger.WithField("stderr", string(errText)).
				Debug("Integration command wrote to stderr.")
		} else {
			ep.logger.Debug("Integration command successful.")
		}
	}
	if commandOk && payloadOk {
		ep.logIfHealthCheck("Integration health check finished with success")
	} else {
		ep.logIfHealthCheck("Integration health check finished with some errors")
	}
}

// handleOutput reads through the lines of output and processes them. It
// returns true if all the lines are processed correctly, false if it cannot
// process any of the lines. In case a line fails, it logs the error and
// continues with the rest.
func (ep *externalPlugin) handleOutput(cmdOut io.ReadCloser, extraLabels data.Map, entityRewrite []data.EntityRewrite) (ok bool) {

	r := bufio.NewReader(cmdOut)
	var line []byte
	lineCount := 0
	ok = true

	for {
		read, more, err := r.ReadLine()
		if err != nil {
			if err != io.EOF {
				ep.logger.WithError(err).WithFields(logrus.Fields{
					"outputLine": line,
				}).Warn("reading integration output")
				ok = false
			}
			if lineCount == 0 {
				ep.logger.Debug("Integration returned no output.")
				ok = false
			}
			break
		}

		line = append(line, read...)
		if !more {
			payloadOk, err := ep.handleLine(line, extraLabels, entityRewrite)
			if err != nil {
				ep.logger.WithError(err).Warn("cannot handle integration output")
			}
			ok = ok && payloadOk
			line = []byte{}
			lineCount++
		}
	}
	return
}

// handleLine unmarshal only as much of the content as it needs to in order
// to figure out which protocol-specific function to route the content to.
// It creates the associated datasets from the payload and emits them.
//
// It returns true if the entire payload is processed without issues, it
// returns false otherwise. If one of the datasets cannot be processed, the
// error is logged and it continues with another.
func (ep *externalPlugin) handleLine(line []byte, extraLabels data.Map, entityRewrite []data.EntityRewrite) (bool, error) {
	pluginData, protocolVersion, err := ParsePayload(line, ep.Context.Config().ForceProtocolV2toV3)
	if err != nil {
		return false, err
	}

	ep.logger.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"payload": string(line),
		}
	}).Debug("Integration payload.")

	ok := true
	for _, dataSet := range pluginData.DataSets {
		key, err := dataSet.Entity.Key()
		if err != nil {
			ok = false
			ep.logger.WithError(err).WithField("entity", dataSet.Entity).Warn("notifying active entity")
		}

		// Reporting to the agent that a given entity is still alive (and active)
		if ep.activeEntities != nil {
			ep.activeEntities <- string(key)
		}

		lbls := make(map[string]string, len(ep.pluginInstance.Labels)+len(extraLabels))
		extraAnnotations := make(map[string]string, len(extraLabels))
		for k, v := range ep.pluginInstance.Labels {
			lbls[k] = v
		}

		for k, v := range extraLabels {
			if strings.HasPrefix(k, labelPrefix) {
				lbls[k[labelPrefixTrim:]] = v
			} else {
				extraAnnotations[k] = v
			}
		}

		err = EmitDataSet(
			ep.Context,
			ep,
			pluginData.Name,
			pluginData.IntegrationVersion,
			ep.pluginInstance.IntegrationUser,
			dataSet,
			extraAnnotations,
			lbls,
			entityRewrite,
			protocolVersion)
		if err != nil {
			ok = false
			ep.logger.WithError(err).Warn("emitting plugin dataset")
		}
	}

	return ok, nil
}

// ParsePayload parses a string containing a JSON payload with the format of our
// SDK for v1, v2 and v3 protocols. Protocol v4 is not supported because this function is
// only used by v3 integration format and older.
func ParsePayload(raw []byte, forceV2ToV3Upgrade bool) (dataV3 protocol.PluginDataV3, protocolVersion int, err error) {
	protocolVersion, err = protocol.VersionFromPayload(raw, forceV2ToV3Upgrade)
	if err != nil {
		return
	}

	dataV3, err = protocol.ParsePayload(raw, protocolVersion)

	return
}

// replaceLoopbackFromField will try to match and replace loopback address from a MetricData field.
func replaceLoopbackFromField(field interface{}, lookup host.IDLookup, protocol int) (string, error) {
	value, ok := field.(string)
	if !ok {
		return "", errors.New("can't replace loopback when the field is not a string")
	}
	return entity.ReplaceLoopback(value, lookup, protocol)
}

func EmitDataSet(
	ctx agent.AgentContext,
	emitter agent.PluginEmitter,
	pluginName string,
	pluginVersion string,
	integrationUser string,
	dataSet protocol.PluginDataSetV3,
	extraAnnotations map[string]string,
	labels map[string]string,
	entityRewrite []data.EntityRewrite,
	protocolVersion int,
) error {
	elog := rlog.WithField("action", "EmitDataSet")

	agentIdentifier := ctx.EntityKey()

	idLookup := ctx.IDLookup()
	entityKey, err := dataSet.Entity.ResolveUniqueEntityKey(agentIdentifier, idLookup, entityRewrite, protocolVersion)
	if err != nil {
		return fmt.Errorf("couldn't determine a unique entity Key: %s", err.Error())
	}

	if len(dataSet.Inventory) > 0 {
		inventoryDataSet := BuildInventoryDataSet(elog, dataSet.Inventory, labels, integrationUser, pluginName, entityKey.String())
		emitter.EmitInventory(inventoryDataSet, entity.NewWithoutID(entityKey))
	}

	for _, metric := range dataSet.Metrics {
		if !dataSet.Entity.IsAgent() {
			if len(metric)+len(extraAnnotations) > maxEntityAttributeCount {
				k := lru.Key(entityKey)
				if _, ok := logLRU.Get(k); !ok {
					elog.
						WithField("entity", entityKey).
						Warn(entityMetricsLengthWarnMgs)
				}
				logLRU.Add(k, struct{}{})
			}
		}

		for key, value := range labels {
			metric[labelPrefix+key] = value
		}
		for key, value := range extraAnnotations {
			// Extra annotations can't override current metrics
			if _, ok := metric[key]; !ok {
				metric[key] = value
			}
		}
		if integrationUser != "" {
			metric["integrationUser"] = integrationUser
		}
		if metricEventType, ok := metric["event_type"]; ok {

			// We want to add displayName and entityName for remote entities in the agent in case these fields are missing
			if !dataSet.Entity.IsAgent() {
				if displayName, ok := metric["displayName"]; !ok || displayName == "" {
					metric["displayName"] = entityKey
				}
				if entityName, ok := metric["entityName"]; !ok || entityName == "" {
					metric["entityName"] = entityKey
				}
				if reportingAgent, ok := metric["reportingAgent"]; !ok || reportingAgent == "" {
					metric["reportingAgent"] = agentIdentifier
				}

				if reportingEndpoint, ok := metric["reportingEndpoint"]; ok {
					replacement, err := replaceLoopbackFromField(reportingEndpoint, idLookup, protocolVersion)
					if err != nil {
						elog.WithError(err).Warn("reportingEndpoint attribute replacement failed")
					} else {
						metric["reportingEndpoint"] = replacement
					}
				}
				if reportingEntityKey, ok := metric["reportingEntityKey"]; ok {
					replacement, err := replaceLoopbackFromField(reportingEntityKey, idLookup, protocolVersion)
					if err != nil {
						elog.WithError(err).Warn("reportingEntityKey attribute replacement failed")
					} else {
						metric["reportingEntityKey"] = replacement
					}
				}
			}
			metric["entityKey"] = entityKey

			// NOTE: The agent requires the eventType field for now
			metric["eventType"] = metricEventType

			metric["integrationName"] = pluginName
			metric["integrationVersion"] = pluginVersion

			// there are integrations that add the hostname so
			// Let's make sure that we do NOT have hostname in the metrics.
			delete(metric, "hostname")

			emitter.EmitEvent(metric, entityKey)
		} else {
			elog.WithIntegration(pluginName).WithField("metric", metric).Debug("Missing event_type field for metric.")
		}
	}

	for _, event := range dataSet.Events {
		normalizedEvent := NormalizeEvent(elog, event, labels, integrationUser, entityKey.String())

		if normalizedEvent != nil {
			emitter.EmitEvent(normalizedEvent, entityKey)
		}
	}

	return nil
}

// hostnameWithLoopbackReplacement is in this case a loopback returns the replacement hostname.
func hostnameWithLoopbackReplacement(hostname interface{}, protocolVersion int, idLookup host.IDLookup) (string, error) {
	h, ok := hostname.(string)
	if !ok {
		return "", errors.New("can't replace loopback hostname when it is not a string")
	}
	if protocolVersion < protocol.V3 {
		return h, nil
	}
	if !http.IsLocalhost(h) {
		return h, nil
	}

	return idLookup.AgentShortEntityName()
}

// cfgTmp is used to store a copy of the configuration to be replaced by discovery/databinding information
type cfgTmp struct {
	CommandLine []string
	Environment map[string]string
}

//  cmdWrapper wraps exec.Cmd with extra labels (metric annotations) from discovery/databinding
type cmdWrapper struct {
	cmd               *exec.Cmd
	entityRewrite     []data.EntityRewrite
	metricAnnotations data.Map
}

// Prepares a command object to run the given PluginCommand. If discovery is enabled, it returns as many
// command instances as discovered items.
//   pluginDir: The directory containing the plugin definition file, used for any relative paths
func (ep *externalPlugin) updateCmdWrappers(pluginDir string) {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	ep.cmdWrappers = []*cmdWrapper{}
	command := ep.pluginCommand

	cfg := cfgTmp{CommandLine: command.Command}
	if ep.pluginInstance.plugin.ProtocolVersion >= protocol.V1 {
		cfg.Environment = ep.envVars()
	} else {
		cfg.Environment = map[string]string{}
	}

	var configs []data.Transformed
	if ep.pluginInstance.plugin.discovery == nil {
		configs = []data.Transformed{{Variables: cfg}}
	} else { // databinding is enabled
		ep.logger.Debug("Fetching discovery/databind sources.")
		vals, err := ep.binder.Fetch(ep.pluginInstance.plugin.discovery)
		if err != nil {
			ep.logger.
				WithError(helpers.ObfuscateSensitiveDataFromError(err)).
				Warn("fetching integration configuration sources")
			return
		}
		configs, err = ep.binder.Replace(&vals, cfg)
		if err != nil {
			ep.logger.
				WithError(helpers.ObfuscateSensitiveDataFromError(err)).
				Warn("data binding process failed. Can't execute integration")
		}
		ep.logger.WithField("matches", len(configs)).Debug("Data binding completed.")
	}
	for _, icfg := range configs {
		rcfg, ok := icfg.Variables.(cfgTmp)
		if !ok {
			// should never ever happen. Just logging so the programmer is aware of a "bug"
			ep.logger.WithField("config", icfg).Warn("unexpected: the plugin configuration should be of type `config`")
			return
		}
		// Convenience/backwards-compatibility:
		// 1 - If the executable path is absolute, use it.
		// 2 - If not absolute and found in the plugin dir, use it.
		// 3 - Otherwise, rely on finding the executable on the PATH.
		executable := rcfg.CommandLine[0]
		if !filepath.IsAbs(executable) {
			if _, err := os.Stat(filepath.Join(pluginDir, executable)); !os.IsNotExist(err) {
				// We were able to locate the given executable relative to the definition file, so give us an absolute path to it
				executable = filepath.Join(pluginDir, executable)
				ep.logger.WithFields(logrus.Fields{
					"relativePath": command.Command[0],
					"absolutePath": executable,
				}).Debug("Converted relative executable to its absolute path.")
			}
		}

		cmd := ep.newCmd(executable, rcfg.CommandLine[1:])
		cmd.Dir = pluginDir

		for k, v := range rcfg.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%v=%v", k, v))
		}
		ep.cmdWrappers = append(ep.cmdWrappers, &cmdWrapper{
			cmd:               cmd,
			metricAnnotations: icfg.MetricAnnotations,
			entityRewrite:     icfg.EntityRewrites,
		})
	}
}

func getVariantHash(entity map[string]interface{}) (hash string, err error) {
	entityBuf, err := json.Marshal(entity)
	if err != nil {
		return "", err
	}

	hashBytes := md5.Sum(entityBuf)
	hash = fmt.Sprintf("%x", hashBytes)
	return
}
