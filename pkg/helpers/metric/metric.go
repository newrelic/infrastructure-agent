// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metric

import (
	"reflect"

	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

const NRDBLimit = 4095

// TruncateLength returns a copy of the "metric" object, whatever its type is,
// with all the strings larger than maxLength cutted down to maxLength
func TruncateLength(event sample.Event, maxLength int) sample.Event {
	if event == nil {
		return nil
	}
	limited, _ := truncRecursive(reflect.ValueOf(event), maxLength)
	return limited.Interface().(sample.Event)
}

// uses reflection to get the type of data that is being passed as metric
// e.g. maps, structs, pointers to maps/structs...
// and then chooses the best strategy to look for strings larger than maxLength
// boolean: returns true if the value has changed
func truncRecursive(val reflect.Value, maxLength int) (reflect.Value, bool) {
	if !val.IsValid() {
		return val, false
	}
	switch val.Kind() {
	case reflect.String:
		return truncString(val, maxLength)
	case reflect.Ptr:
		if val.IsNil() {
			return val, false
		}
		return truncPointer(val, maxLength)
	case reflect.Interface:
		if val.IsNil() {
			return val, false
		}
		return truncInterface(val, maxLength)
	case reflect.Map:
		if val.IsNil() {
			return val, false
		}
		return truncMap(val, maxLength)
	case reflect.Struct:
		return truncStruct(val, maxLength)
	default:
		return val, false
	}
}

func truncString(val reflect.Value, maxLength int) (reflect.Value, bool) {
	if val.Len() > maxLength {
		return reflect.ValueOf(val.String()[:maxLength]), true
	}
	return val, false
}

func truncStruct(val reflect.Value, maxLength int) (reflect.Value, bool) {
	newStruct := reflect.New(val.Type()).Elem()
	anyChanged := false
	for i := 0; i < val.NumField(); i++ {
		limited, changed := truncRecursive(val.Field(i), maxLength)
		newStruct.Field(i).Set(limited)
		anyChanged = anyChanged || changed
	}
	if anyChanged {
		return newStruct, true
	}
	return val, false
}

// if we can cast the value to a simple map[string]interface{}, we iterate it as a normal
// go map, as it is around 5-6 times faster than iterating it via reflection
// if not, we use reflection to iterate a map whatever its type
func truncMap(val reflect.Value, maxLength int) (reflect.Value, bool) {
	if m, ok := val.Interface().(map[string]interface{}); ok {
		return truncGoMap(m, maxLength, val)
	}
	return truncReflectionMap(val, maxLength)
}

func truncReflectionMap(val reflect.Value, maxLength int) (reflect.Value, bool) {
	keys := val.MapKeys()
	anyChanged := false
	for _, k := range keys {
		if limited, changed := truncRecursive(val.MapIndex(k), maxLength); changed {
			val.SetMapIndex(k, limited)
			anyChanged = true
		}
	}
	return val, anyChanged
}

func truncGoMap(m map[string]interface{}, maxLength int, val reflect.Value) (reflect.Value, bool) {
	anyChanged := false
	for k, v := range m {
		switch s := v.(type) {
		case string:
			if len(s) > maxLength {
				m[k] = s[:maxLength]
				anyChanged = true
			}
		case *string:
			if len(*s) > maxLength {
				cut := (*s)[:maxLength]
				m[k] = &cut
				anyChanged = true
			}
		}
	}
	if anyChanged {
		val = reflect.ValueOf(m)
	}
	return val, anyChanged
}

func truncInterface(val reflect.Value, maxLength int) (reflect.Value, bool) {
	limited, changed := truncRecursive(reflect.ValueOf(val.Interface()), maxLength)
	if !changed {
		return val, false
	}
	return limited, true
}

func truncPointer(val reflect.Value, maxLength int) (reflect.Value, bool) {
	limited, changed := truncRecursive(val.Elem(), maxLength)
	if !changed {
		return val, false
	}
	if limited.CanAddr() {
		return limited.Addr(), true
	}
	if val.Elem().Kind() == reflect.String {
		a := limited.String()
		return reflect.ValueOf(&a), true
	}
	a := limited.Interface()
	return reflect.ValueOf(&a), true
}
