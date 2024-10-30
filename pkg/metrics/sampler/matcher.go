// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
)

var (
	mlog = log.WithComponent("SamplerMatcher")

	// typesToEvaluate is map that contains the samples we want to filter on
	typesToEvaluate = map[string]bool{
		"ProcessSample":     true, // Normal process sample
		"FlatProcessSample": true, // Process sample combined with Docker process data
	}
)

// MatcherFn func that returns whether an event/sample is matched. It satisfies
// the metrics matcher (processor.MatcherChain) interface.
type MatcherFn func(event any) bool

// IncludeSampleMatchFn func that returns whether an event/sample should be included, it satisfies
// the metrics matcher (processor.MatcherChain) interface.
type IncludeSampleMatchFn MatcherFn

// ExcludeSampleMatchFn func that returns whether an event/sample should be excluded, it satisfies
// the metrics matcher (processor.MatcherChain) interface.
type ExcludeSampleMatchFn MatcherFn

// ExpressionMatcher is an interface every evaluator must implement
type ExpressionMatcher interface {
	// Evaluate compare property value with evaluator criteria and
	// return if it found a match
	Evaluate(event interface{}) bool
}

type attributeCache map[string][]string

var attrCache attributeCache

type regexCompiledCache map[string]*regexp.Regexp

var regexCache regexCompiledCache

func init() {
	attrCache = attributeCache{
		"process.name": []string{
			"ProcessDisplayName", // Field name from ProcessSample
			"processDisplayName", // Field name from FlatProcessSample (i.e. the map key name)
		},
		"process.executable": []string{
			"CmdLine",     // Field name from ProcessSample
			"commandLine", // Field name from FlatProcessSample (i.e. the map key name)
		},
	}
	regexCache = regexCompiledCache{}
}

type matcher struct {
	PropertyName  []string
	ExpectedValue interface{}
	Evaluator     func(expected interface{}, actual interface{}) bool
}

func (p matcher) Evaluate(event interface{}) bool {
	actualValue := getFieldValue(event, p.PropertyName)
	if actualValue == nil {
		return false
	}
	isMatch := p.Evaluator(p.ExpectedValue, actualValue)
	mlog.
		WithField(config.TracesFieldName, config.FeatureTrace).
		Tracef("'%v' matches expression %v >> '%v': %v", actualValue, p.PropertyName, p.ExpectedValue, isMatch)
	return isMatch
}

func getMapValue(object map[string]interface{}, fieldNames []string) interface{} {
	mlog.
		WithField(config.TracesFieldName, config.FeatureTrace).
		Tracef("Searching map[string]interface{} for fields %v", fieldNames)
	for i := range fieldNames {
		obj := object[fieldNames[i]]
		if obj != nil {
			return obj
		}
	}
	return nil
}

func getFieldValue(object interface{}, fieldNames []string) interface{} {
	v := reflect.ValueOf(object)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// If a struct then try and get it by field
	if v.Kind() == reflect.Struct {
		mlog.
			WithField(config.TracesFieldName, config.FeatureTrace).
			Tracef("Searching Struct for fields %v", fieldNames)
		for i := range fieldNames {
			fv := v.FieldByName(fieldNames[i])
			if fv.IsValid() && fv.CanInterface() {
				fieldValue := fv.Interface()
				return fieldValue
			}
		}
	}

	// Anything else then work out the type
	switch v.Interface().(type) {
	case types.FlatProcessSample: // types.FlatProcessSample is a map so check if any of the field names contains a value
		return getMapValue(v.Interface().(types.FlatProcessSample), fieldNames)
	default:
		mlog.
			WithField(config.TracesFieldName, config.FeatureTrace).
			Tracef("Fields %v does NOT exist in sample.", fieldNames)
		return nil
	}
}

func literalExpressionEvaluator(expected interface{}, actual interface{}) bool {
	return expected == actual
}

func regularExpressionEvaluator(expected interface{}, actual interface{}) bool {
	// regex should never be nil here.
	regex := regexCache[expected.(string)]
	return regex.MatchString(fmt.Sprintf("%v", actual))
}

// newExpressionMatcher returns a new ExpressionMatcher
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
//
// - process.executable
//   - "/bin/test"
//   - regex "^/opt/newrelic/"
//
// will create an evaluator chain with 2 entries. The first one will have 1 evaluator. The second 2 evaluators
type MatcherChain struct {
	Matchers map[string][]ExpressionMatcher
	Enabled  bool
}

// NewMatcherChain creates a new chain of matchers.
// Each expression will generate an matcher that gets added to the chain
// While the chain will be matched for each "sample", it terminates as soon as 1 match is matched (result = true)
func NewMatcherChain(expressions config.MetricsMap) MatcherChain {
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
//   - true, if event match with evaluator criteria chain
//   - false, if event do not match with evaluator criteria chain
//
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

// NewIncludeSampleMatchFn returns a function `func(sample) bool` that determinesif the sample
// should be included (true) as an event or not (false). Note that this is NOT the negation
// of `NewExcludeSampleMatchFn`.
func NewIncludeSampleMatchFn(enableProcessMetrics *bool, metricsMatchers config.IncludeMetricsMap, ffRetriever feature_flags.Retriever) MatcherFn {
	return func(sample any) bool {
		// We return early if the sample is not a ProcessSample.
		if !isProcessSample(sample) {
			// Include this event
			return true
		}

		// Continuing with the logic
		// configuration option always takes precedence over FF and matchers configuration
		if enableProcessMetrics == nil {
			matcher := matcherFromMetricsMatchers(config.MetricsMap(metricsMatchers))
			if matcher != nil {
				return matcher(sample)
			}

			// configuration option is not defined and feature flag is present, FF determines, otherwise
			// all process samples will be excluded. Note that this is not exactly the negation
			// of `NewExcludeSampleMatchFn`.
			enabled, exists := ffRetriever.GetFeatureFlag(fflag.FlagFullProcess)

			return exists && enabled
		}

		if !*enableProcessMetrics {
			mlog.
				WithField(config.TracesFieldName, config.FeatureTrace).
				Trace("EnableProcessMetrics is FALSE, process metrics will be DISABLED")

			return false
		}

		matcher := matcherFromMetricsMatchers(config.MetricsMap(metricsMatchers))
		if matcher != nil {
			return matcher(sample)
		}

		mlog.
			WithField(config.TracesFieldName, config.FeatureTrace).
			Trace("EnableProcessMetrics is TRUE and rules are NOT defined, ALL process metrics will be ENABLED")

		return true
	}
}

// NewExcludeSampleMatchFn returns a function `func(sample) bool` that determinesif the sample
// should be excluded (true) as an event or not (false). Note that this is NOT the negation
// of `NewIncludeSampleMatchFn`. In particular, we don't check here for the `enableProcessMetrics`
// being unset or disabled because it is checked before calling this function at `agent.NewAgent`.
func NewExcludeSampleMatchFn(metricsMatchers config.ExcludeMetricsMap) MatcherFn {
	return func(sample any) bool {
		// We return early if the sample is not a ProcessSample.
		if !isProcessSample(sample) {
			// Do NOT exclude this event
			return false
		}

		matcher := matcherFromMetricsMatchers(config.MetricsMap(metricsMatchers))
		if matcher != nil {
			return matcher(sample)
		}

		mlog.
			WithField(config.TracesFieldName, config.FeatureTrace).
			Trace("EnableProcessMetrics is TRUE and rules are NOT defined, ALL process metrics will be ENABLED")

		return false
	}
}

func matcherFromMetricsMatchers(metricsMatchers config.MetricsMap) MatcherFn {
	// if config option is not set, check if we have rules defined. those take precedence over the FF
	matcherChain := NewMatcherChain(metricsMatchers)
	if matcherChain.Enabled {
		mlog.
			WithField(config.TracesFieldName, config.FeatureTrace).
			Tracef("EnableProcessMetrics is EMPTY and rules ARE defined, process metrics will be ENABLED for matching processes")

		return matcherChain.Evaluate
	}

	return nil
}

func isProcessSample(sample any) bool {
	switch sample.(type) {
	case *types.ProcessSample, *types.FlatProcessSample:
		return true
	default:
		return false
	}
}
