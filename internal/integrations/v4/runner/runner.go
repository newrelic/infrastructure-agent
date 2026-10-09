// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/instrumentation"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/constants"
	"github.com/newrelic/infrastructure-agent/pkg/config" //nolint:depguard
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/cache"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/health"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/when"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/contexts"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	cfgprotocol "github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/sirupsen/logrus"
)

var (
	illog         = log.WithComponent("integrations.runner.Runner")
	heartBeatJSON = []byte("{}")
	//A Logrus line is expected to be a list of key-value pairs separated by an equal character. Keys can contain any
	//character, whereas values can have three different formats:
	//1- string: <quote>any character including escaped quotes \"<quote>
	//2- map: &{any character}
	//3- word: any character except spaces
	logrusRegexp = regexp.MustCompile(`([^\s]*?)=(".*?[^\\]"|&{.*?}|[^\s]*)`)
	sdkLogRegexp = regexp.MustCompile(`\[([^\]]*)\]\s*(.+)`)
)

// generic types to handle the stderr log parsing
type logFields map[string]interface{}
type logParser func(line string) (fields logFields)

// runner for a single integration entry
type runner struct {
	emitter        emitter.Emitter
	handleCmdReq   cmdrequest.HandleFn
	handleConfig   configrequest.HandleFn
	dSources       *databind.Sources
	log            log.Entry
	definition     integration.Definition
	handleErrors   func(context.Context, <-chan error) // by default, runner.logErrors. Replaceable for testing purposes
	stderrParser   []logParser
	lastStderr     stderrQueue
	healthCheck    sync.Once
	heartBeatFunc  func()
	heartBeatMutex sync.RWMutex
	cache          cache.Cache
	terminateQueue chan<- string
	idLookup       host.IDLookup
	// healthReporter is nil unless integrations health reporting is enabled, which is the default.
	healthReporter *health.Reporter
}

// NewRunner creates an integration runner instance.
// args: discoverySources, handleErrorsProvide and cmdReqHandle are optional (nils allowed).
func NewRunner(
	intDef integration.Definition,
	emitter emitter.Emitter,
	dSources *databind.Sources,
	handleErrorsProvide func() runnerErrorHandler,
	cmdReqHandle cmdrequest.HandleFn,
	configHandle configrequest.HandleFn,
	terminateQ chan<- string,
	idLookup host.IDLookup,
) *runner {
	r := &runner{
		emitter:        emitter,
		handleCmdReq:   cmdReqHandle,
		handleConfig:   configHandle,
		dSources:       dSources,
		definition:     intDef,
		heartBeatFunc:  func() {},
		stderrParser:   []logParser{parseSDKFields, parseLogrusFields},
		lastStderr:     newStderrQueue(intDef.LogsQueueSize),
		terminateQueue: terminateQ,
		cache:          cache.CreateCache(),
		idLookup:       idLookup,
	}
	if handleErrorsProvide != nil {
		r.handleErrors = handleErrorsProvide()
	} else {
		r.handleErrors = r.logErrors
	}

	return r
}

// WithHealthReporter sets the reporter this runner records its execution results into, and returns
// the runner so it can be chained onto NewRunner. A nil reporter, the default, disables health
// recording entirely.
func (r *runner) WithHealthReporter(hr *health.Reporter) *runner { //nolint:revive
	r.healthReporter = hr

	return r
}

// recordHealth stores the outcome of an execution round. It is a no-op when health reporting is
// disabled, and a write failure never interrupts the integration.
func (r *runner) recordHealth(res health.Result) {
	if r.healthReporter == nil {
		return
	}

	if err := r.healthReporter.Record(r.definition.Hash(), r.definition.Name, res); err != nil {
		r.log.WithError(err).Warn("Cannot write integration health file")
	}
}

// lineOutcome is what one integration process' standard output produced, from a health standpoint.
type lineOutcome struct {
	dataPayloads int
	errs         []string
}

func (r *runner) Run(ctx context.Context, pidWCh, exitCodeCh chan<- int) {
	r.log = illog.WithFields(LogFields(r.definition))
	defer r.killChildren()
	for {
		waitForNextExecution := time.After(r.definition.Interval)

		// only cmd-channel run-requests require exit-code, and they only trigger a single instance
		//var exitCodeCh chan int
		//if r.definition.RequiresEvent() {
		//	exitCodeCh = make(chan int, 1)
		//}

		discovery, info, err := r.applyDiscovery()
		if err != nil {
			r.log.
				WithError(helpers.ObfuscateSensitiveDataFromError(err)).
				Error("can't fetch discovery items")
			r.recordHealth(health.Result{
				Executions: 1,
				Errors:     []string{helpers.ObfuscateSensitiveDataFromError(err).Error()},
			})
		} else {
			if when.All(r.definition.WhenConditions...) {
				r.execute(ctx, discovery, info, pidWCh, exitCodeCh)
			} else {
				r.log.Debug("Integration conditions where not met, skipping execution")
				// nothing ran, which is not a failure of the integration
				r.recordHealth(health.Result{})
			}
		}

		if r.definition.SingleRun() {
			r.log.Debug("Integration single run finished")
			return
		}

		select {
		case <-ctx.Done():
			r.log.Debug("Integration has been interrupted")
			return
		case <-waitForNextExecution:
		}
	}
}

func (r *runner) killChildren() {
	if c := r.cache; c != nil {
		cfgNames := c.ListConfigNames()
		for _, cfgName := range cfgNames {
			definitions := r.cache.GetDefinitions(cfgName)
			for _, d := range definitions {
				r.terminateQueue <- d.Hash()

				var cfgName string
				if d.CfgProtocol != nil {
					cfgName = d.CfgProtocol.ConfigName
				}
				r.log.
					WithField("child_name", d.Name).
					WithField("cfg_protocol_name", cfgName).
					Debug("Stopping child integration")
			}
		}
	}
}

func LogFields(def integration.Definition) logrus.Fields {
	fields := logrus.Fields{
		"integration_name": def.Name,
	}
	for k, v := range def.Labels {
		fields[k] = v
	}

	for k, v := range def.Tags {
		fields[k] = v
	}

	if def.CfgProtocol != nil {
		fields["cfg_protocol_name"] = def.CfgProtocol.ConfigName
		fields["parent_integration_name"] = def.CfgProtocol.ParentName
	}

	fields["runner_uid"] = def.Hash()[:10]

	return fields
}

// applies dSources and returns the discovered values, if any.
func (r *runner) applyDiscovery() (*databind.Values, databind.DiscovererInfo, error) {
	if r.dSources == nil {
		// nothing is discovered, but the integration can run (with the default configuration)
		return nil, databind.DiscovererInfo{}, nil
	}
	if v, err := databind.Fetch(r.dSources); err != nil {
		return nil, r.dSources.Info, err
	} else {
		return &v, r.dSources.Info, nil
	}
}

// set the heartBeatFunc to use.
// This function will make sure the heartBeatFunc isn't replaced while being executed by another go routine
func (r *runner) setHeartBeat(heartBeatFunc func()) {
	r.heartBeatMutex.Lock()
	defer r.heartBeatMutex.Unlock()
	r.heartBeatFunc = heartBeatFunc
}

// heartBeat triggers heartBeatFunc
// This functions makes sure we access heartBeatFunc in a thread-safe manner
func (r *runner) heartBeat() {
	r.heartBeatMutex.RLock()
	defer r.heartBeatMutex.RUnlock()
	r.heartBeatFunc()
}

// execute the integration and wait for all the possible instances (resulting of multiple dSources matches)
// to finish
// For long-time running integrations, avoids starting the next
// discover-execute cycle until all the parallel processes have ended
func (r *runner) execute(ctx context.Context, matches *databind.Values, discoveryInfo databind.DiscovererInfo, pidWCh, exitCodeCh chan<- int) {
	ctx, txn := instrumentation.SelfInstrumentation.StartTransaction(ctx, "integration.v4."+r.definition.Name)
	if hostname, ok := r.definition.ExecutorConfig.Environment["HOSTNAME"]; ok {
		txn.AddAttribute("integration_hostname", hostname)
	}
	if port, ok := r.definition.ExecutorConfig.Environment["PORT"]; ok {
		txn.AddAttribute("integration_port", port)
	}

	defer txn.End()
	def := r.definition

	// If timeout configuration is set, wraps current context in a heartbeat-enabled timeout context
	if def.TimeoutEnabled() {
		var act contexts.Actuator
		ctx, act = contexts.WithHeartBeat(ctx, def.Timeout, r.log)
		r.setHeartBeat(act.HeartBeat)
		defer act.HeartBeatStop()
	}

	// add hostID in the context to fetch and set in executor
	hostID, err := r.idLookup.AgentShortEntityName()

	if err == nil {
		ctx = contextWithHostID(ctx, hostID)
	} else {
		txn.NoticeError(err)
		r.log.WithError(err).Error("can't fetch host ID")
	}

	// Runs all the matching integration instances
	outputs, err := r.definition.Run(ctx, matches, discoveryInfo, pidWCh, exitCodeCh)
	if err != nil {
		txn.NoticeError(err)
		r.log.WithError(err).Error("can't start integration")
		r.recordHealth(health.Result{
			Executions: 1,
			Errors:     []string{helpers.ObfuscateSensitiveDataFromError(err).Error()},
		})

		return
	}

	// Waits for all the integrations to finish and reads the standard output and errors
	wg := sync.WaitGroup{}
	waitForCurrent := make(chan struct{})
	wg.Add(len(outputs) * 3)

	// each goroutine writes only its own index, so these need no lock; they must not be read before
	// waitForCurrent is closed
	lineOutcomes := make([]lineOutcome, len(outputs))
	errOutcomes := make([][]string, len(outputs))

	for i, out := range outputs {
		i, o := i, out
		go func(txn instrumentation.Transaction) {
			defer wg.Done()
			lineOutcomes[i] = r.handleLines(ctx, o.Receive.Stdout, o.ExtraLabels, o.EntityRewrite)
		}(txn)

		go func(txn instrumentation.Transaction) {
			defer wg.Done()
			r.handleStderr(o.Receive.Stderr)
		}(txn)

		go func(txn instrumentation.Transaction) {
			defer wg.Done()
			errOutcomes[i] = r.handleErrorsAndCollect(ctx, o.Receive.Errors)
		}(txn)
	}

	r.log.Debug("Waiting while the integration instances run.")
	go func() {
		wg.Wait()
		close(waitForCurrent)
	}()

	select {
	case <-ctx.Done():
		r.log.Debug("Integration has been interrupted. Finishing.")
		// the goroutines above may still be writing to lineOutcomes and errOutcomes, so report a
		// fixed result instead of reading them
		r.recordHealth(health.Result{
			Executions: len(outputs),
			Errors:     []string{"integration execution was interrupted or timed out"},
		})
	case <-waitForCurrent:
		r.log.Debug("Integration instances finished their execution. Waiting until next interval.")
		r.recordHealth(roundResult(lineOutcomes, errOutcomes))
	}
}

// roundResult collapses the per-process outcomes of one execution round into a single result.
func roundResult(lineOutcomes []lineOutcome, errOutcomes [][]string) health.Result {
	res := health.Result{Executions: len(lineOutcomes)}

	for _, outcome := range lineOutcomes {
		res.DataPayloads += outcome.dataPayloads
		res.Errors = append(res.Errors, outcome.errs...)
	}

	for _, errs := range errOutcomes {
		res.Errors = append(res.Errors, errs...)
	}

	return res
}

func (r *runner) handleStderr(stderr <-chan []byte) {
	for line := range stderr {
		r.lastStderr.Add(line)

		// obfuscated stderr
		obfuscatedLine := helpers.ObfuscateSensitiveDataFromString(string(line))
		var lineWasParsed bool

		for _, errParser := range r.stderrParser {
			fields := errParser(obfuscatedLine)

			if lvl, ok := fields["level"]; ok {
				// If a field already exists, like the time, logrus automatically adds the prefix "deps." to the
				// Duplicated keys
				logLine := r.log.WithFields(logrus.Fields(fields))
				logMessage := "integration log"

				lineWasParsed = true

				if r.log.IsDebugEnabled() {
					// If Debug is enabled the level is ignored.
					logLine.Debug(logMessage)

					break
				}

				lvl, ok := lvl.(string)
				if ok {
					switch strings.ToLower(lvl) {
					// We only want to respect Fatal and Error levels.
					case "info", "debug", "warn", "warning":
						logLine.Debug(logMessage)
					case "trace":
						logLine.Trace(logMessage)
					default:
						// This error log line has the filtered-by-default "integration-errors" component.
						// Unless included on the config, this log lines won't be logged by the agent.
						logLine.WithComponent(config.IntegrationsErrorsValue).Error(logMessage)
					}

					break
				}
			}
		}

		if !lineWasParsed {
			r.log.WithField("line", obfuscatedLine).Debug("Integration stderr (not parsed).")
		}
	}
}

// implementation of the "handleErrors" property
func (r *runner) logErrors(ctx context.Context, errs <-chan error) {
	for {
		select {
		case <-ctx.Done():
			// don't log errors if the context has been just cancelled
			return
		case err := <-errs:
			if err == nil {
				// channel closed: exiting
				return
			}
			flush := r.lastStderr.Flush()
			// err contains the exit code number
			r.log.WithError(err).WithField("stderr", helpers.ObfuscateSensitiveDataFromString(flush)).
				Warn("integration exited with error state")
		}
	}
}

// handleErrorsAndCollect forwards the execution error channel to the configured error handler and,
// when health reporting is enabled, collects the obfuscated error messages on the way. Draining
// errs to completion is what guarantees the forwarding goroutine terminates: the executor closes
// the channel when the process ends.
func (r *runner) handleErrorsAndCollect(ctx context.Context, errs <-chan error) []string {
	if r.healthReporter == nil {
		r.handleErrors(ctx, errs)

		return nil
	}

	var collected []string

	forwarded := make(chan error)
	handlerDone := make(chan struct{})

	go func() {
		defer close(handlerDone)
		r.handleErrors(ctx, forwarded)
	}()

	for err := range errs {
		if err != nil {
			collected = append(collected, helpers.ObfuscateSensitiveDataFromError(err).Error())
		}

		select {
		case forwarded <- err:
		case <-handlerDone:
			// the handler returned early, most likely because the context was cancelled; keep
			// draining errs so the executor is never blocked on a full channel
		}
	}

	close(forwarded)
	<-handlerDone

	return collected
}

func (r *runner) handleLines(ctx context.Context, stdout <-chan []byte, extraLabels data.Map, entityRewrite []data.EntityRewrite) lineOutcome {
	txn := instrumentation.TransactionFromContext(ctx)
	payloadSize := 0
	outcome := lineOutcome{}
	for line := range stdout {
		llog := r.log.WithFieldsF(func() logrus.Fields {
			return logrus.Fields{"payload": string(line)}
		})

		if isHeartBeat(line) {
			llog.Debug("Received heartbeat.")
			r.heartBeat()
			continue
		}

		if ok, ver := protocol.IsCommandRequest(line); ok {
			if r.handleCmdReq == nil {
				llog.Warn("received cmd request payload without a handler")
				continue
			}
			llog.WithField("version", ver).Debug("Received run request.")
			cr, err := protocol.DeserializeLine(line)
			if err != nil {
				llog.
					WithError(err).
					Warn("cannot deserialize integration run request payload")
				continue
			}

			r.handleCmdReq(cr, r.definition)
			continue
		}

		if cfgProtocolBuilder := cfgprotocol.GetConfigProtocolBuilder(line); cfgProtocolBuilder != nil {
			// obfuscate config protocol output
			obfuscatedllog := r.log.WithFieldsF(func() logrus.Fields {
				return logrus.Fields{"payload": helpers.ObfuscateSensitiveDataFromString(string(line))}
			})

			if r.handleConfig == nil {
				obfuscatedllog.Warn("received config protocol request payload without a handler")
				continue
			}
			cfgProtocol, err := cfgProtocolBuilder.Build()
			obfuscatedllog.WithField("version", cfgProtocol.Version()).Debug("Received config protocol request.")
			if err != nil {
				obfuscatedllog.
					WithError(err).
					Warn("cannot build config protocol")
				continue
			}

			r.handleConfig(cfgProtocol, r.cache, r.definition)
			continue
		}

		payloadSize += len(line)
		if r.healthReporter != nil && health.HasData(line) {
			outcome.dataPayloads++
		}

		err := r.emitter.Emit(r.definition, extraLabels, entityRewrite, line)
		if err != nil {
			llog.WithError(err).Warn("Cannot emit integration payload")

			if r.healthReporter != nil {
				outcome.errs = append(outcome.errs, helpers.ObfuscateSensitiveDataFromError(err).Error())
			}
		} else {
			r.heartBeat()
		}

		r.healthCheck.Do(func() {
			if err == nil {
				r.log.Info("Integration health check finished with success")
			} else {
				r.log.WithError(err).Warn("Integration health check finished with some errors")
			}
		})
	}

	txn.AddAttribute("payload_size", payloadSize)

	return outcome
}

func contextWithHostID(ctx context.Context, hostID string) context.Context {
	return context.WithValue(ctx, constants.HostID, hostID)
}

func isHeartBeat(line []byte) bool {
	return bytes.Equal(bytes.Trim(line, " "), heartBeatJSON)
}

func parseLogrusFields(line string) (fields logFields) {
	matches := logrusRegexp.FindAllStringSubmatch(line, -1)
	fields = make(logFields, len(matches))

	for _, m := range matches {
		key, val := m[1], m[2]
		if strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
			// remove both quotes from the string
			val = val[1 : len(val)-1]
		}
		fields[key] = val
	}

	return
}

func parseSDKFields(line string) logFields {
	matches := sdkLogRegexp.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil
	}

	lvl, msg := matches[0][1], matches[0][2]

	level := strings.ToLower(lvl)
	if level == "err" {
		level = "error"
	}

	return logFields{
		"level": level,
		"msg":   msg,
	}
}
