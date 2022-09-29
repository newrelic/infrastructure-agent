// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/sirupsen/logrus"
)

// FilteringFormatterConfig is the configuration used to instantiate a new FilteringFormatter.
type FilteringFormatterConfig struct {
	IncludeFilters config.LogFilters
	ExcludeFilters config.LogFilters
}

// FilteringFormatter decorator implementing logrus.Formatter interface.
// It is a wrapper around a given formatter adding filtering by log entries keys.
type FilteringFormatter struct {
	includeFilters logEntryMatcher
	excludeFilters logEntryMatcher

	wrapped logrus.Formatter
}

// NewFilteringFormatter creates a new FilteringFormatter.
func NewFilteringFormatter(cfg FilteringFormatterConfig, wrapped logrus.Formatter) *FilteringFormatter {
	return &FilteringFormatter{
		includeFilters: newLogEntryMatcher(cfg.IncludeFilters),
		excludeFilters: newLogEntryMatcher(cfg.ExcludeFilters),
		wrapped:        wrapped,
	}
}

// Format renders a single log entry.
func (f *FilteringFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Include has precedence over exclude configuration.
	// A log line will bypass filtering only if IS NOT Excluded or if IS Included.
	bypassFiltering := !f.excludeFilters.match(entry) || f.includeFilters.match(entry)

	if bypassFiltering {
		return f.wrapped.Format(entry)
	}
	return nil, nil
}

// logEntryMatcher will try to match key-value pairs.
type logEntryMatcher map[string]map[interface{}]struct{}

// newLogEntryMatcher creates a new logEntryMatcher from a map[string][]interface{}
// In order to match key-value pair easily will convert []interface{} into a set.
func newLogEntryMatcher(logFilters config.LogFilters) logEntryMatcher {
	matcher := make(map[string]map[interface{}]struct{}, len(logFilters))
	for key, value := range logFilters {
		set := make(map[interface{}]struct{})

		for _, value := range value {
			if !isTypeSupported(value) {
				continue
			}
			set[value] = struct{}{}
		}

		if len(set) > 0 || key == config.LogFilterWildcard {
			matcher[key] = set
		}
	}

	return matcher
}

// match returns true if the entry contains the fields specified by the filter configuration.
func (l logEntryMatcher) match(entry *logrus.Entry) bool {
	if len(l) == 0 {
		return false
	}

	// If wildcard is configured, match everything.
	if _, found := l[config.LogFilterWildcard]; found {
		return true
	}

	for key, value := range entry.Data {
		if !isTypeSupported(value) {
			continue
		}

		if _, found := l[key][config.LogFilterWildcard]; found {
			return true
		}

		if _, ok := l[key][value]; ok {
			return true
		}
	}
	return false
}

// isTypeSupported asserts if the object is supported. We want to avoid using objects that cannot be used
// as a key in a map.
func isTypeSupported(obj interface{}) bool {
	switch obj.(type) {
	case string, int:
		return true
	default:
		return false
	}
}
