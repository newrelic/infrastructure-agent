package format

import (
	"github.com/sirupsen/logrus"
)

// wrapper around a given formatter to filter by log entries keys
type FieldFormatter struct {
	fieldsSet map[string]map[interface{}]struct{}
	wrap      logrus.Formatter
}

func NewFieldFormatter(fields map[string][]interface{}, wrappedFormatter logrus.Formatter) *FieldFormatter {
	sets := make(map[string]map[interface{}]struct{}, len(fields))
	for key, values := range fields {
		sets[key] = make(map[interface{}]struct{})
		for _, value := range values {
			switch value.(type) {
			// filter string and int types to prevent runtime errors with incomparable types
			case int, string:
				sets[key][value] = struct{}{}
			}
		}
	}
	return &FieldFormatter{sets, wrappedFormatter}
}

// Format renders a single log entry
func (f *FieldFormatter) Format(entry *logrus.Entry) ([]byte, error) {
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
