package format

import (
	"github.com/sirupsen/logrus"
)

// wrapper around a given formatter to filter by log entries keys
type FieldFormatter struct {
	fieldsSet map[string]map[interface{}]struct{}
	include   bool
	wrap      logrus.Formatter
}

// setFromMap creates a "set" structure from a given map
func setFromMap(m map[string][]interface{}) map[string]map[interface{}]struct{} {
	set := make(map[string]map[interface{}]struct{}, len(m))
	for key, values := range m {
		set[key] = make(map[interface{}]struct{})
		for _, value := range values {
			switch value.(type) {
			// filter string and int types to prevent runtime errors with incomparable types
			case int, string:
				set[key][value] = struct{}{}
			}
		}
	}
	return set
}

func NewFieldFormatter(fields map[string][]interface{}, include bool, wrappedFormatter logrus.Formatter) *FieldFormatter {
	return &FieldFormatter{setFromMap(fields), include, wrappedFormatter}
}

// Format renders a single log entry
func (f *FieldFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	if f.include {
		return f.includeFilter(entry)
	}
	return f.excludeFilter(entry)
}

// includeFilter skips the entries that do not contain any of the defined filters
func (f *FieldFormatter) includeFilter(entry *logrus.Entry) ([]byte, error) {
	for key, value := range entry.Data {
		switch value.(type) {
		case int, string:
			if _, ok := f.fieldsSet[key][value]; ok {
				return f.wrap.Format(entry)
			}
		}
	}
	return nil, nil
}

// excludeFilter skips the entries that contain any of the defined filters
func (f *FieldFormatter) excludeFilter(entry *logrus.Entry) ([]byte, error) {
	for key, value := range entry.Data {
		switch value.(type) {
		case int, string:
			if _, ok := f.fieldsSet[key][value]; ok {
				return nil, nil
			}
		}
	}
	return f.wrap.Format(entry)
}
