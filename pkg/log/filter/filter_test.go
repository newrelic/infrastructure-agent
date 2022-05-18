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
		Name         string
		config       FilteringFormatterConfig
		Entry        *logrus.Entry
		ExpectedLine string
	}{
		{
			Name:         "WhenNoFilters_ReturnLine",
			config:       FilteringFormatterConfig{},
			Entry:        logrus.WithField("component", 1),
			ExpectedLine: "time=\"0001-01-01T00:00:00Z\" level=panic component=1\n",
		},
		{
			Name: "WhenMatchingIncludeFilters_ReturnsLine",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"component": {1, "Plugin", 1},
				},
			},
			Entry:        logrus.WithField("component", 1).WithField("component2", "value"),
			ExpectedLine: "time=\"0001-01-01T00:00:00Z\" level=panic component=1 component2=value\n",
		},
		{
			Name: "WhenNotMatchingIncludeFilters_ReturnEmpty",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"unknown": {"test", 3.2},
				},
			},
			Entry:        logrus.WithField("component", 1).WithField("component2", "value"),
			ExpectedLine: "",
		},
		{
			Name: "WhenMatchingIncludeAndExcludeFilters_ReturnEmpty",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"component":  {1},
					"component2": {"value"},
				},
				ExcludeFilters: map[string][]interface{}{
					"component2": {"value"},
				},
			},
			Entry:        logrus.WithField("component", 1).WithField("component2", "value"),
			ExpectedLine: "",
		},
		{
			Name: "WhenMatchingIncludeAndExcludeFilters_WithIncludePrecedence_ReturnLine",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"component":  {1},
					"component2": {"value"},
				},
				ExcludeFilters: map[string][]interface{}{
					"component2": {"value"},
				},
				IncludePrecedence: true,
			},
			Entry:        logrus.WithField("component", 1).WithField("component2", "value"),
			ExpectedLine: "time=\"0001-01-01T00:00:00Z\" level=panic component=1 component2=value\n",
		},
		{
			Name: "WhenMatchingExcludeFilters_WithIncludePrecedence_ReturnEmpty",
			config: FilteringFormatterConfig{
				IncludeFilters: map[string][]interface{}{
					"unknown": {1},
				},
				ExcludeFilters: map[string][]interface{}{
					"component2": {"value"},
				},
				IncludePrecedence: true,
			},
			Entry:        logrus.WithField("component", 1).WithField("component2", "value"),
			ExpectedLine: "",
		},
		{
			Name: "UnsupportedConfig_ReturnEmpty",
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
				IncludePrecedence: true,
			},
			Entry:        logrus.WithField("component", []string{"value1", "value2"}),
			ExpectedLine: "time=\"0001-01-01T00:00:00Z\" level=panic component=\"[value1 value2]\"\n",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			var textFormatter logrus.Formatter = &logrus.TextFormatter{}
			formatter := NewFilteringFormatter(testCase.config, textFormatter)

			actualEntry, err := formatter.Format(testCase.Entry)
			assert.NoError(t, err)
			assert.Equal(t, testCase.ExpectedLine, string(actualEntry))
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
