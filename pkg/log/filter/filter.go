package filter

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/sirupsen/logrus"
)

// FilteringFormatterConfig is the configuration used to instantiate a new FilteringFormatter.
type FilteringFormatterConfig struct {
	IncludeFilters    config.LogFilters
	ExcludeFilters    config.LogFilters
	IncludePrecedence bool
}

// FilteringFormatter decorator implementing logrus.Formatter interface.
// It is a wrapper around a given formatter adding filtering by log entries keys.
type FilteringFormatter struct {
	includeFilters logEntryMatcher
	excludeFilters logEntryMatcher

	// includePrecedence specifies if include filters should have more priority.
	includePrecedence bool

	wrapped logrus.Formatter
}

// NewFilteringFormatter creates a new FilteringFormatter.
func NewFilteringFormatter(cfg FilteringFormatterConfig, wrapped logrus.Formatter) *FilteringFormatter {
	return &FilteringFormatter{
		includeFilters:    newLogEntryMatcher(cfg.IncludeFilters),
		excludeFilters:    newLogEntryMatcher(cfg.ExcludeFilters),
		includePrecedence: cfg.IncludePrecedence,
		wrapped:           wrapped,
	}
}

// Format renders a single log entry.
func (f *FilteringFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// When no include filters are specified, everything is included.
	includeAll := len(f.includeFilters) == 0

	shouldInclude := includeAll || f.includeFilters.match(entry)
	if !shouldInclude {
		return nil, nil
	}

	if !f.includePrecedence && len(f.excludeFilters) > 0 && f.excludeFilters.match(entry) {
		return nil, nil
	}

	return f.wrapped.Format(entry)
}

// logEntryMatcher will try to match key-value pairs.
type logEntryMatcher map[string]map[interface{}]struct{}

// newLogEntryMatcher creates a new logEntryMatcher from a map[string][]interface{}
// In order to match key-value pair easily will convert []interface{} into a set.
func newLogEntryMatcher(logFilters config.LogFilters) logEntryMatcher {
	matcher := make(map[string]map[interface{}]struct{}, len(logFilters))
	for key, values := range logFilters {
		matcher[key] = make(map[interface{}]struct{})
		for _, value := range values {
			matcher[key][value] = struct{}{}
		}
	}
	return matcher
}

// match returns true if the entry has the fields specified by the filter configuration.
func (l logEntryMatcher) match(entry *logrus.Entry) bool {
	for key, value := range entry.Data {
		if _, ok := l[key][value]; ok {
			return true
		}
	}
	return false
}
