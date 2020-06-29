// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
	"reflect"
	"regexp"
	"strings"
)

var (
	typesToEvaluate = map[string]bool{"ProcessSample": true}
)

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
		return constantMatcher{false}
	}

	eval := matcher{
		PropertyName: mappedAttributeName,
	}

	if strings.HasPrefix(expr, "regex") {
		regex := strings.Trim(strings.TrimSpace(strings.TrimLeft(expr, "regex")), `"`)
		cacheRegex(regex)
		eval.ExpectedValue = regex
		eval.Evaluator = regularExpressionEvaluator
	} else {
		eval.ExpectedValue = strings.TrimSpace(strings.Trim(expr, `"`))
		eval.Evaluator = literalExpressionEvaluator
	}

	return eval
}

func cacheRegex(regex string) {
	//if not cached yet, cache it
	if _, ok := regexCache[regex]; !ok {
		regexCache[regex] = regexp.MustCompile(regex)
	}
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
func NewMatcherChain(expressions map[string][]string) MatcherChain {
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
