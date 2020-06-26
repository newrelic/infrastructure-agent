// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"fmt"
	"strconv"
)

type Map map[string]string
type InterfaceMap map[string]interface{}

type Transformed struct {
	Variables         interface{}
	MetricAnnotations Map
	EntityRewrites    []EntityRewrite
}

type EntityRewrite struct {
	Action       string `json:"action"`
	Match        string `json:"match"`
	ReplaceField string `json:"replaceField"`
}

func InterfaceMapToMap(original InterfaceMap) (out Map) {
	out = make(Map, len(original))
	AddValues(out, "", original)
	return out
}

type GenericDiscovery struct {
	Variables      InterfaceMap    `json:"variables"`
	Annotations    InterfaceMap    `json:"metricAnnotations"`
	EntityRewrites []EntityRewrite `json:"entityRewrites"`
}

// Adds a structured value to a flat map, where each key has a
// JS-like notation to access fields or arrays, if the value is
// structured
// e.g. if val is a map {"prop":{"arr":[1,2,3]}} it would add
// the following entries to the destination map
// "prop.arr[0]" : "1"
// "prop.arr[1]" : "2"
// "prop.arr[2]" : "3"
// please note that int values are converted to string
func AddValues(dst Map, prefix string, val interface{}) {
	var pfx string
	if prefix != "" {
		pfx = prefix + "."
	} else {
		pfx = ""
	}
	switch value := val.(type) {
	case string:
		dst[prefix] = value
	case map[string]string:
		for k, v := range value {
			dst[pfx+k] = v
		}
	case map[string]interface{}:
		for k, v := range value {
			AddValues(dst, pfx+k, v)
		}
	case InterfaceMap:
		for k, v := range value {
			AddValues(dst, pfx+k, v)
		}
	case []string:
		for k, v := range value {
			dst[prefix+"["+strconv.Itoa(k)+"]"] = v
		}
	case []interface{}:
		for k, v := range value {
			AddValues(dst, prefix+"["+strconv.Itoa(k)+"]", v)
		}
	default:
		dst[prefix] = fmt.Sprintf("%+v", value)
	}
}
