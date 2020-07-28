// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"reflect"
	"regexp"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

var (
	typesToEvaluate = map[string]bool{"ProcessSample": true}
)

var mlog = log.WithComponent("SamplerMatcher")

// IncludeSampleMatchFn func that returns whether an event/sample should be included, it satisfies
// the metrics matcher (processor.MatcherChain) interface.
type IncludeSampleMatchFn func(sample interface{}) bool

// ExpressionMatcher is an interface every evaluator must implement
type ExpressionMatcher interface {
	// Evaluate compare property value with evaluator criteria and
	// return if it found a match
	Evaluate(event interface{}) bool
}

type attributeCache map[string]string

var attrCache attributeCache

type regexCompiledCache map[string]*regexp.Regexp

var regexCache regexCompiledCache

func init() {
	attrCache = attributeCache{
		"process.name":       "ProcessDisplayName",
		"process.executable": "CmdLine",
	}
	regexCache = regexCompiledCache{}
}

type matcher struct {
	PropertyName  string
	ExpectedValue interface{}
	Evaluator     func(expected interface{}, actual interface{}) bool
}

func (p matcher) Evaluate(event interface{}) bool {
	if skipSample(event, typesToEvaluate) {
		return true
	}

	actualValue := getFieldValue(event, p.PropertyName)
	if actualValue == nil {
		return false
	}
	isMatch := p.Evaluator(p.ExpectedValue, actualValue)
	trace.MetricMatch("'%v' matches expression '%v' >> '%v': %v", actualValue, p.PropertyName, p.ExpectedValue, isMatch)
	return isMatch
}

func getFieldValue(object interface{}, fieldName string) interface{} {
	v := reflect.ValueOf(object)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	fv := v.FieldByName(fieldName)
	if fv.IsValid() && fv.CanInterface() {
		fieldValue := fv.Interface()
		return fieldValue
	}

	trace.MetricMatch("field '%v' does NOT exist in sample", fieldName)
	return nil
}

// determine is this is a sample that should be evaluated
// we are only checking ProcessSamples at this point, so only those should be evaluated
func skipSample(sample interface{}, typesToEvaluate map[string]bool) bool {
	v := reflect.TypeOf(sample)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	// skip if not "registered"
	return !typesToEvaluate[v.Name()]
}

func literalExpressionEvaluator(expected interface{}, actual interface{}) bool {
	return expected == actual
}

func regularExpressionEvaluator(expected interface{}, actual interface{}) bool {
	// regex should never be nil here.
	regex := regexCache[expected.(string)]
	return regex.MatchString(fmt.Sprintf("%v", actual))
}

//newExpressionMatcher returns a new ExpressionMatcher
func newExpressionMatcher(dimensionName string, expr string) ExpressionMatcher {
	return build(dimensionName, expr)
}

func build(dimensionName string, expr string) ExpressionMatcher {
	// if the dimension is not "registered", return a constant "false" matcher
	// "false" will make the chain continue (until either there is a "true" result or there's no more matchers),
	// so this matcher basically get's ignored in the current implementation
	mappedAttributeName, found := attrCache[dimensionName]
	if !found {
		return constantMatcher{value: false}
	}

	eval := matcher{
		PropertyName: mappedAttributeName,
	}

	if strings.HasPrefix(expr, "regex") {
		regex := strings.Trim(strings.TrimSpace(strings.TrimLeft(expr, "regex")), `"`)
		if err := cacheRegex(regex); err != nil {
			mlog.WithError(err).Error(fmt.Sprintf("could not intitilize expression matcher for the provided configuration: '%s'", expr))
			return constantMatcher{value: false}
		}
		eval.ExpectedValue = regex
		eval.Evaluator = regularExpressionEvaluator
	} else {
		eval.ExpectedValue = strings.TrimSpace(strings.Trim(expr, `"`))
		eval.Evaluator = literalExpressionEvaluator
	}

	return eval
}

func cacheRegex(pattern string) error {
	//if not cached yet, cache it
	if _, ok := regexCache[pattern]; !ok {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
		regexCache[pattern] = regex
	}
	return nil
}

// MatcherChain is a chain of evaluators
// An evaluator chain stores for each attribute an array of evaluators
// Each evaluator represent a single rule that evaluates against the attribute
// Usually each attribute will have a single evaluator.
// For example:
// - process.name
//   - "test"
// - process.executable
//   - "/bin/test"
//   - regex "^/opt/newrelic/"
// will create an evaluator chain with 2 entries. The first one will have 1 evaluator. The second 2 evaluators
type MatcherChain struct {
	Matchers map[string][]ExpressionMatcher
	Enabled  bool
}

// NewMatcherChain creates a new chain of matchers.
// Each expression will generate an matcher that gets added to the chain
// While the chain will be matched for each "sample", it terminates as soon as 1 match is matched (result = true)
func NewMatcherChain(expressions config.IncludeMetricsMap) MatcherChain {
	chain := MatcherChain{Matchers: map[string][]ExpressionMatcher{}, Enabled: false}

	// no matchers means the chain will be disabled
	if len(expressions) == 0 {
		return chain
	}

	chain.Enabled = true
	for prop, exprs := range expressions {
		if _, ok := chain.Matchers[prop]; !ok {
			chain.Matchers[prop] = []ExpressionMatcher{}
		}

		evs := chain.Matchers[prop]
		for _, expr := range exprs {
			e := newExpressionMatcher(prop, expr)
			evs = append(evs, e)
		}
		chain.Matchers[prop] = evs
	}

	return chain
}

// Evaluate returns the result of compare an event with a chain of matching rules
// return:
//  - true, if event match with evaluator criteria chain
//  - false, if event do not match with evaluator criteria chain
// If there is no matchers will return true.
func (ec MatcherChain) Evaluate(event interface{}) bool {
	var result = true
	for _, es := range ec.Matchers {
		for _, e := range es {
			result = e.Evaluate(event)
			if result {
				return result
			}
		}
	}
	return result
}

type constantMatcher struct {
	value bool
}

func (ne constantMatcher) Evaluate(_ interface{}) bool {
	return ne.value
}
func (ne constantMatcher) String() string {
	return fmt.Sprint(ne.value)
}

// NewSampleMatchFn creates new includeSampleMatchFn func, enableProcessMetrics might be nil when
// value was not set.
func NewSampleMatchFn(enableProcessMetrics *bool, includeMetricsMatchers config.IncludeMetricsMap, ffRetriever feature_flags.Retriever) IncludeSampleMatchFn {
	// configuration option always takes precedence over FF and matchers configuration
	if enableProcessMetrics != nil {
		if *enableProcessMetrics == false {
			trace.MetricMatch("EnableProcessMetrics is FALSE, process metrics will be DISABLED")
			return func(sample interface{}) bool {
				// no process samples are included
				_, isProcessSample := sample.(*types.ProcessSample)
				return !isProcessSample
			}
		} else {
			ec := NewMatcherChain(includeMetricsMatchers)
			if ec.Enabled {
				trace.MetricMatch("EnableProcessMetrics is TRUE and rules ARE defined, process metrics will be ENABLED for matching processes")
				return func(sample interface{}) bool {
					return ec.Evaluate(sample)
				}
			}

			trace.MetricMatch("EnableProcessMetrics is TRUE and rules are NOT defined, ALL process metrics will be ENABLED")
			return func(sample interface{}) bool {
				// all process samples are included
				return true
			}
		}
	}

	// if config option is not set, check if we have rules defined. those take precedence over the FF
	ec := NewMatcherChain(includeMetricsMatchers)
	if ec.Enabled {
		trace.MetricMatch("EnableProcessMetrics is EMPTY and rules ARE defined, process metrics will be ENABLED for matching processes")
		return func(sample interface{}) bool {
			return ec.Evaluate(sample)
		}
	}

	// configuration option is not defined and feature flag is present, FF determines, otherwise
	// all process samples will be excluded
	return func(sample interface{}) bool {
		if _, isProcessSample := sample.(*types.ProcessSample); !isProcessSample {
			return true
		}

		enabled, exists := ffRetriever.GetFeatureFlag(handler.FlagFullProcess)
		return exists && enabled
	}
}
