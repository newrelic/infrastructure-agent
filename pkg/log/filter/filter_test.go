// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestNewLogEntryMatcher(t *testing.T) {
	fields := map[string][]interface{}{
		"component": {1, "Plugin", 8908},
	}
	entryMatcher := newLogEntryMatcher(fields)
	assert.Equal(t, 3, len(entryMatcher["component"]))
	assert.Contains(t, entryMatcher["component"], "Plugin")
	assert.Contains(t, entryMatcher["component"], 1)
	assert.Contains(t, entryMatcher["component"], 8908)
}

func TestFormat(t *testing.T) {
	testCases := []struct {
		Name          string
		config        FilteringFormatterConfig
		Entries       []*logrus.Entry
		ExpectedLines []string
	}{
		{
			Name:   "WhenEmptyConfig_ReturnLine",
			config: FilteringFormatterConfig{},
			Entries: []*logrus.Entry{
				logrus.WithField("component", 1),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic component=1\n",
			},
		},
		{
			Name: "WhenNotMatchingIncludeOrExcludeFilters_ReturnLine",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"unknown": {"test", 3.2},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", 1).WithField("component2", "value"),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic component=1 component2=value\n",
			},
		},
		{
			Name: "WhenMatchingIncludeFilters_ReturnsLine",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"component": {1, "Plugin", 1},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", 1).WithField("component2", "value"),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic component=1 component2=value\n",
			},
		},
		{
			Name: "WhenMatchingExcludeButNotIncludeFilters_ReturnsEmpty",
			config: FilteringFormatterConfig{
				ExcludeFilters: map[string][]interface{}{
					"component": {1, "Plugin", 1},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", 1).WithField("component2", "value"),
			},
			ExpectedLines: []string{
				"",
			},
		},
		{
			Name: "WhenMatchingExcludeAndIncludeFilters_ReturnLine",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"component":  {1},
					"component2": {"value"},
				},
				ExcludeFilters: map[string][]interface{}{
					"component2": {"value"},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", 1).WithField("component2", "value"),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic component=1 component2=value\n",
			},
		},
		{
			Name: "WhenUnsupportedConfig_ReturnEmpty",
			config: FilteringFormatterConfig{
				ExcludeFilters: map[string][]interface{}{
					"unknown": {
						// slices are not accepted as keys in map.
						[]string{"value1", "value2"},
					},
				},
				IncludeFilters: map[string][]interface{}{
					"unknown": {
						// slices are not accepted as keys in map.
						[]string{"value1", "value2"},
					},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", []string{"value1", "value2"}),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic component=\"[value1 value2]\"\n",
			},
		},
		{
			Name: "WhenWildcardKeyProvided_FiltersEverything",
			config: FilteringFormatterConfig{
				ExcludeFilters: map[string][]interface{}{
					"*": nil,
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", "component_value"),
				logrus.WithField("component2", "component_value2"),
			},
			ExpectedLines: []string{
				"",
				"",
			},
		},
		{
			Name: "WhenWildcardForValueProvided_FiltersOnlyValues",
			config: FilteringFormatterConfig{
				ExcludeFilters: map[string][]interface{}{
					"filter_this": {
						"*",
					},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", "component_value"),
				logrus.WithField("filter_this", "value1"),
				logrus.WithField("filter_this", "value2"),
				logrus.WithField("filter_this", "value3"),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic component=component_value\n",
				"",
				"",
				"",
			},
		},
		{
			Name: "WhenFilterOneFieldValue_TheRestAreReturned",
			config: FilteringFormatterConfig{
				ExcludeFilters: map[string][]interface{}{
					"filter_this": {
						"value2",
					},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("component", "component_value"),
				logrus.WithField("filter_this", "value1"),
				logrus.WithField("filter_this", "value2"),
				logrus.WithField("filter_this", "value3"),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic component=component_value\n",
				"time=\"0001-01-01T00:00:00Z\" level=panic filter_this=value1\n",
				"",
				"time=\"0001-01-01T00:00:00Z\" level=panic filter_this=value3\n",
			},
		},
		{
			Name: "WhenWildcardInIncluded_ReturnsEverything",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"*": nil,
				},
				ExcludeFilters: map[string][]interface{}{
					"traces": {
						"supervisor",
					},
				},
			},
			Entries: []*logrus.Entry{
				logrus.WithField("trace", "supervisor"),
			},
			ExpectedLines: []string{
				"time=\"0001-01-01T00:00:00Z\" level=panic trace=supervisor\n",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			var textFormatter logrus.Formatter = &logrus.TextFormatter{}
			formatter := NewFilteringFormatter(testCase.config, textFormatter)

			for i, entry := range testCase.Entries {
				actualEntry, err := formatter.Format(entry)
				assert.NoError(t, err)
				assert.Equal(t, testCase.ExpectedLines[i], string(actualEntry))
			}
		})
	}
}

func Benchmark(b *testing.B) {
	var tests = []struct {
		name           string
		includeFilters map[string][]interface{}
		excludeFilters map[string][]interface{}
	}{
		{
			name: "Enabled",
			includeFilters: map[string][]interface{}{
				"component": {1, "Plugin", 8908}},
		},
		{
			name:           "Disabled",
			includeFilters: nil,
		},
	}
	for _, t := range tests {
		b.Run(t.name, func(b *testing.B) {
			logger := logrus.New()
			logger.SetFormatter(&logrus.TextFormatter{})
			if t.includeFilters != nil {
				formatterConfig := FilteringFormatterConfig{
					IncludeFilters: t.includeFilters,
				}
				formatter := NewFilteringFormatter(formatterConfig, logger.Formatter)
				logger.SetFormatter(formatter)
			}
			benchmarkLogger(b, logger, []logrus.Fields{
				{"some": "fields", "component": "Plugin", "here": struct{}{}},
				{"no": "matching", "fields": 1}})
		})
	}
}

func benchmarkLogger(b *testing.B, l *logrus.Logger, fields []logrus.Fields) {
	l.SetLevel(logrus.DebugLevel)
	l.SetOutput(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		for _, logFields := range fields {
			l.WithFields(logFields).Debug("This message won't be displayed.")
		}
	}
}
