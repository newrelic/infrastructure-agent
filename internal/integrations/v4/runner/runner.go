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
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/when"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/contexts"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest/protocol"
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
)

//generic types to handle the stderr log parsing
type logFields map[string]interface{}
type logParser func(line string) (fields logFields)

// runner for a single integration entry
type runner struct {
	emitter        emitter.Emitter
	handleCmdReq   cmdrequest.HandleFn
	dSources       *databind.Sources
	log            log.Entry
	definition     integration.Definition
	handleErrors   func(context.Context, <-chan error) // by default, runner.logErrors. Replaceable for testing purposes
	stderrParser   logParser
	lastStderr     stderrQueue
	healthCheck    sync.Once
	heartBeatFunc  func()
	heartBeatMutex sync.RWMutex
}

// NewRunner creates an integration runner instance.
// args: discoverySources, handleErrorsProvide and cmdReqHandle are optional (nils allowed).
func NewRunner(
	intDef integration.Definition,
	emitter emitter.Emitter,
	dSources *databind.Sources,
	handleErrorsProvide func() runnerErrorHandler,
	cmdReqHandle cmdrequest.HandleFn,
) *runner {
	r := &runner{
		emitter:       emitter,
		handleCmdReq:  cmdReqHandle,
		dSources:      dSources,
		definition:    intDef,
		heartBeatFunc: func() {},
		stderrParser:  parseLogrusFields,
	}
	if handleErrorsProvide != nil {
		r.handleErrors = handleErrorsProvide()
	} else {
		r.handleErrors = r.logErrors
	}

	return r
}

func (r *runner) Run(ctx context.Context, pidWCh, exitCodeCh chan<- int) {
	r.log = illog.WithFields(LogFields(r.definition))
	for {
		waitForNextExecution := time.After(r.definition.Interval)

		// only cmd-channel run-requests require exit-code, and they only trigger a single instance
		//var exitCodeCh chan int
		//if r.definition.RequiresEvent() {
		//	exitCodeCh = make(chan int, 1)
		//}

		values, err := r.applyDiscovery()
		if err != nil {
			r.log.
				WithError(helpers.ObfuscateSensitiveDataFromError(err)).
				Error("can't fetch discovery items")
		} else {
			if when.All(r.definition.WhenConditions...) {
				r.execute(ctx, values, pidWCh, exitCodeCh)
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

func LogFields(def integration.Definition) logrus.Fields {
	fields := logrus.Fields{
		"integration_name": def.Name,
	}
	for k, v := range def.Labels {
		fields[k] = v
	}
	return fields
}

// applies dSources and returns the discovered values, if any.
func (r *runner) applyDiscovery() (*databind.Values, error) {
	if r.dSources == nil {
		// nothing is discovered, but the integration can run (with the default configuration)
		return nil, nil
	}
	if v, err := databind.Fetch(r.dSources); err != nil {
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
func (r *runner) execute(ctx context.Context, matches *databind.Values, pidWCh, exitCodeCh chan<- int) {
	def := r.definition

	// If timeout configuration is set, wraps current context in a heartbeat-enabled timeout context
	if def.TimeoutEnabled() {
		var act contexts.Actuator
		ctx, act = contexts.WithHeartBeat(ctx, def.Timeout)
		r.setHeartBeat(act.HeartBeat)
	}

	// Runs all the matching integration instances
	outputs, err := r.definition.Run(ctx, matches, pidWCh, exitCodeCh)
	if err != nil {
		r.log.WithError(err).Error("can't start integration")
		return
	}

	// Waits for all the integrations to finish and reads the standard output and errors
	wg := sync.WaitGroup{}
	waitForCurrent := make(chan struct{})
	wg.Add(len(outputs))
	for _, out := range outputs {
		o := out
		go r.handleLines(o.Receive.Stdout, o.ExtraLabels, o.EntityRewrite)
		go r.handleStderr(o.Receive.Stderr)
		go func() {
			defer wg.Done()
			r.handleErrors(ctx, o.Receive.Errors)
		}()
	}

	r.log.Debug("Waiting while the integration instances run.")
	go func() {
		wg.Wait()
		close(waitForCurrent)
	}()

	select {
	case <-ctx.Done():
		r.log.Debug("Integration has been interrupted. Finishing.")
	case <-waitForCurrent:
		r.log.Debug("Integration instances finished their execution. Waiting until next interval.")
	}

	return
}

func (r *runner) handleStderr(stderr <-chan []byte) {
	for line := range stderr {
		r.lastStderr.Add(line)

		if r.log.IsDebugEnabled() {
			r.log.WithField("line", string(line)).Debug("Integration stderr (not parsed).")
		} else {
			fields := r.stderrParser(string(line))
			if v, ok := fields["level"]; ok && (v == "error" || v == "fatal") {
				// If a field already exists, like the time, logrus automatically adds the prefix "deps." to the
				// Duplicated keys
				r.log.WithFields(logrus.Fields(fields)).Error("received an integration log line")
			}
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

		if ok, ver := protocol.IsCommandRequest(line); ok {
			llog.WithField("version", ver).Debug("Received run request.")
			cr, err := protocol.DeserializeLine(line)
			if err != nil {
				llog.
					WithError(err).
					Warn("cannot deserialize integration run request payload")
				continue
			}

			if r.handleCmdReq == nil {
				llog.Warn("received cmd request payload without a handler")
				continue
			}

			r.handleCmdReq(cr)
			continue
		}

		err := r.emitter.Emit(r.definition, extraLabels, entityRewrite, line)
		if err != nil {
			llog.WithError(err).Warn("Cannot emit integration payload")
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
