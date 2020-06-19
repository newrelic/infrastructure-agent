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

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/v3legacy"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/when"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/contexts"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/sirupsen/logrus"
)

var illog = log.WithComponent("integrations.runner.Group")

var heartBeatJSON = []byte("{}")

//generic types to handle the stderr log parsing
type logFields map[string]interface{}
type logParser func(line string) (fields logFields)

//A Logrus line is expected to be a list of key-value pairs separated by an equal character. Keys can contain any
//character, whereas values can have three different formats:
//1- string: <quote>any character including escaped quotes \"<quote>
//2- map: &{any character}
//3- word: any character except spaces
var logrusRegexp = regexp.MustCompile(`([^\s]*?)=(".*?[^\\]"|&{.*?}|[^\s]*)`)

// Group represents a set of runnable integrations that are located in
// the same integration configuration file, and thus share a common
// discovery mechanism configuration. It also does the following tasks:
// - parses integration output and forwards it
// - parses standard error and logs it
// - catches errors and logs them
// - manages the cancellation of tasks, as this should-be hot-reloaded
type Group struct {
	discovery    *databind.Sources
	integrations []integration.Definition
	emitter      emitter.Emitter
	definitions  *v3legacy.DefinitionsRepo
	// for testing purposes, allows defining which action to take when an execution
	// error is received. If unset, it will be runner.logErrors
	getErrorHandler func(r *runner) runnerErrorHandler
}

type runnerErrorHandler func(errs <-chan error)

func sendErrorsToLog(r *runner) runnerErrorHandler {
	return r.logErrors
}

// Run launches all the integrations to run in background. They can be cancelled with the
// provided context
func (t *Group) Run(ctx context.Context) (hasStartedAnyOHI bool) {
	if t.getErrorHandler == nil {
		t.getErrorHandler = sendErrorsToLog
	}
	for _, integr := range t.integrations {
		r := runner{
			parent:        t,
			Integration:   integr,
			heartBeatFunc: func() {},
			stderrParser:  parseLogrusFields,
		}
		r.handleErrors = t.getErrorHandler(&r)
		go r.Run(ctx)
		hasStartedAnyOHI = true
	}

	return
}

// runner for a single integration entry
type runner struct {
	ctx            context.Context // to avoid logging too many errors when the integration is cancelled by the user
	parent         *Group
	log            log.Entry
	Integration    integration.Definition
	handleErrors   func(<-chan error) // by default, runner.logErrors. Replaceable for testing purposes
	stderrParser   logParser
	lastStderr     stderrQueue
	healthCheck    sync.Once
	heartBeatFunc  func()
	heartBeatMutex sync.RWMutex
}

func (r *runner) Run(ctx context.Context) {
	r.ctx = ctx
	config := r.Integration
	fields := logrus.Fields{
		"integration_name": config.Name,
	}
	for k, v := range config.Labels {
		fields[k] = v
	}
	r.log = illog.WithFields(fields)
	for {
		// we start counting the interval time on each integration execution
		waitForNextExecution := time.After(config.Interval)

		values, err := r.applyDiscovery()
		if err != nil {
			r.log.
				WithError(
					helpers.ObfuscateSensitiveDataFromError(err)).
				Error("can't fetch discovery items")
		} else {
			// the integration runs only if all the when: conditions are true, if any
			if when.All(r.Integration.WhenConditions...) {
				r.execute(ctx, values)
			}
		}

		select {
		case <-ctx.Done():
			r.log.Debug("Integration has been interrupted. Finishing.")
			return
		case <-waitForNextExecution:
		}
	}
}

// applies discovery and returns the discovered values, if any.
func (r *runner) applyDiscovery() (*databind.Values, error) {
	if r.parent.discovery == nil {
		// nothing is discovered, but the integration can run (with the default configuration)
		return nil, nil
	}
	if v, err := databind.Fetch(r.parent.discovery); err != nil {
		return nil, err
	} else {
		return &v, nil
	}
}

// set the heartBeatFunc to use.
// This function will make sure the heartBeatFunc isn't replaced while being executed by another go routine
func (r *runner) setHeartBeat(heartBeatFunc func()) {
	r.heartBeatMutex.Lock()
	defer r.heartBeatMutex.Unlock()
	r.heartBeatFunc = heartBeatFunc
}

// execute the heartBeatFunc
// This functions makes sure we access heartBeatFunc in a thread-safe manner
func (r *runner) heartBeat() {
	r.heartBeatMutex.RLock()
	defer r.heartBeatMutex.RUnlock()
	r.heartBeatFunc()
}

// execute the integration and wait for all the possible instances (resulting of multiple discovery matches)
// to finish
// For long-time running integrations, avoids starting the next
// discover-execute cycle until all the parallel processes have ended
func (r *runner) execute(ctx context.Context, matches *databind.Values) {
	config := r.Integration

	// If timeout configuration is set, wraps current context in a heartbeat-enabled timeout context
	if config.TimeoutEnabled() {
		var act contexts.Actuator
		ctx, act = contexts.WithHeartBeat(ctx, config.Timeout)
		r.setHeartBeat(act.HeartBeat)
	}

	// Runs all the matching integration instances
	output, err := r.Integration.Run(ctx, matches)
	if err != nil {
		r.log.WithError(err).Error("can't start integration")
		return
	}

	// Waits for all the integrations to finish and reads the standard output and errors
	instances := sync.WaitGroup{}
	waitForCurrent := make(chan struct{})
	instances.Add(len(output))
	for _, out := range output {
		o := out
		go r.handleLines(o.Output.Stdout, o.ExtraLabels, o.EntityRewrite)
		go r.handleStderr(o.Output.Stderr)
		go func() {
			defer instances.Done()
			r.handleErrors(o.Output.Errors)
		}()
	}

	r.log.Debug("Waiting while the integration instances run.")
	go func() {
		instances.Wait()
		close(waitForCurrent)
	}()

	select {
	case <-ctx.Done():
		r.log.Debug("Integration has been interrupted. Finishing.")
		return
	case <-waitForCurrent:
	}

	r.log.Debug("Integration instances finished their execution. Waiting until next interval.")
}

func (r *runner) handleStderr(stderr <-chan []byte) {
	for line := range stderr {
		r.lastStderr.Add(line)

		if r.log.IsDebugEnabled() {
			r.log.Debug(string(line))
		} else {
			fields := r.stderrParser(string(line))
			if v, ok := fields["level"]; ok && (v == "error" || v == "fatal") {
				// If a field already exists, like the time, logrus automatically adds the prefix "fields." to the
				// Duplicated keys
				r.log.WithFields(logrus.Fields(fields)).Error("received an integration log line")
			}
		}
	}
}

// implementation of the "handleErrors" property
func (r *runner) logErrors(errs <-chan error) {
	for {
		select {
		case <-r.ctx.Done():
			// don't log errors if the context has been just cancelled
			return
		case err := <-errs:
			if err == nil {
				// channel closed: exiting
				return
			}
			flush := r.lastStderr.Flush()
			r.log.WithError(err).WithField("stderr", flush).
				Warn("integration exited with error state")
		}
	}
}

func (r *runner) handleLines(stdout <-chan []byte, extraLabels data.Map, entityRewrite []data.EntityRewrite) {
	for line := range stdout {
		llog := r.log.WithFieldsF(func() logrus.Fields {
			return logrus.Fields{"payload": string(line)}
		})

		if isHeartBeat(line) {
			llog.Debug("Received heartbeat.")
			r.heartBeat()
			continue
		}

		llog.Debug("Received payload.")
		err := r.parent.emitter.Emit(r.Integration, extraLabels, entityRewrite, line)
		if err != nil {
			llog.WithError(err).Warn("can't emit integration payloads")
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
