package format

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestNewFieldFormatter(t *testing.T) {
	fields := map[string][]interface{}{
		"component": {1, "Plugin", 8908},
	}
	var textFormatter logrus.Formatter = &logrus.TextFormatter{}
	formatter := NewFieldFormatter(fields, textFormatter)
	assert.Equal(t, 3, len(formatter.fieldsSet["component"]))
	assert.Contains(t, formatter.fieldsSet["component"], "Plugin")
	assert.Contains(t, formatter.fieldsSet["component"], 1)
	assert.Contains(t, formatter.fieldsSet["component"], 8908)
}

func TestFormatter(t *testing.T) {
	fields := map[string][]interface{}{
		"component": {1, "Plugin", 8908},
	}
	var textFormatter logrus.Formatter = &logrus.TextFormatter{}
	formatter := NewFieldFormatter(fields, textFormatter)
	actualEntry, err := formatter.Format(logrus.WithField("component", 1))
	assert.NoError(t, err)
	assert.NotNil(t, actualEntry)

	actualEntry, err = formatter.Format(logrus.WithField("component", "1"))
	assert.Nil(t, err)
	assert.Nil(t, actualEntry)
}

func Benchmark(b *testing.B) {
	var tests = []struct {
		name    string
		filters map[string][]interface{}
	}{
		{"Enabled", map[string][]interface{}{"component": {1, "Plugin", 8908}}},
		{"Disabled", nil},
	}
	for _, t := range tests {
		b.Run(t.name, func(b *testing.B) {
			logger := logrus.New()
			logger.SetFormatter(&logrus.TextFormatter{})
			if t.filters != nil {
				formatter := NewFieldFormatter(t.filters, logger.Formatter)
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
