// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"regexp"
)

type FieldsMatcher struct {
	matcher map[string]matchingFunc
}

type matchingFunc func(val string) bool

func stringEquals(matcher string) matchingFunc {
	return func(val string) bool {
		return val == matcher
	}
}

func regexpMatch(regexp *regexp.Regexp) matchingFunc {
	return func(val string) bool {
		return regexp.MatchString(val)
	}
}

// we'll identify any regular expression as a string between two slashes (as Ruby lang)
var metaRegexp = regexp.MustCompile("^/.*/$")

func NewMatcher(fieldMatchers map[string]string) (FieldsMatcher, error) {
	cm := FieldsMatcher{
		matcher: map[string]matchingFunc{},
	}

	for field, str := range fieldMatchers {
		if metaRegexp.MatchString(str) {
			// the matcher is a regular expression. Remove the delimiting slashes and compiling
			regex := str[1 : len(str)-1]
			cmp, err := regexp.Compile(regex)
			if err != nil {
				return cm, fmt.Errorf("value of %q should be a valid regular expression: %s", field, err.Error())
			}
			cm.matcher[field] = regexpMatch(cmp)
		} else {
			cm.matcher[field] = stringEquals(str)
		}
	}
	return cm, nil
}

func (cm *FieldsMatcher) All(fields map[string]string) bool {
	for field, matchFunc := range cm.matcher {
		if val, ok := fields[field]; !ok || !matchFunc(val) {
			return false
		}
	}
	return true
}

func LabelsToMap(prefix string, labels map[string]string) data.Map {
	ret := make(data.Map, len(labels))
	for key, value := range labels {
		ret[prefix+key] = value
	}
	return ret
}
