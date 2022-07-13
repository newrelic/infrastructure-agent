// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"bytes"
	"reflect"
	"regexp"
	"unsafe"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/naming"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

type replaceConfig struct {
	onDemand []OnDemand
}

// Option provide extra behaviour configuration to the replacement process.
type ReplaceOption func(rc *replaceConfig)

// This regular expression matches any variable mark ${...} with dots and index marks [ ]
var regex = regexp.MustCompile(`\$\{[\w\d\._\s\[\]-]*\}`)

// Replace receives one template, which may be a map or a struct whose string fields may
// contain ${variable} placeholders, and returns an array of items of the same type of the
// template, but replacing the variable placeholders from the respective Values.
// The Values of type "variable" are the same for all the returned values. The returned
// array contains one instance per each "discovered" data value.
func Replace(vals *Values, template interface{}, options ...ReplaceOption) (transformedData []data.Transformed, err error) {
	rc := replaceConfig{}
	for _, option := range options {
		option(&rc)
	}
	// if neither discovery nor variables, we just return the template as it
	if len(vals.discov) == 0 {
		if len(vals.vars) == 0 {
			//Historycally used to check for variables in template and return error in that case
			//Now variables in template are allowed as they can be used in flex i.e and rc is meant
			//to be executed internal/integrations/v4/integration/definition.go
			//for onDemand = ignoreConfigPathVar(&foundConfigPath)
			replaceAllSources(template, []discovery.Discovery{{}}, data.Map{}, rc)

			// otherwise, it means it does not have variables, so we will return the template as it was
			// since it was not bounded to any discovery process
			return []data.Transformed{{Variables: template}}, nil
		}
		// if no discovery data but variables, we just replace variables as if they were
		// a discovery source and leave the "common" values as empty
		return replaceAllSources(template, []discovery.Discovery{{Variables: vals.vars}}, data.Map{}, rc), nil
	}

	discoverySources := vals.discov
	varSrc := vals.vars

	return replaceAllSources(template, discoverySources, varSrc, rc), nil
}

// Replaces all the discovery sources by the values in the "src" array. The common array is shared
// If src is empty, no change is done even if there is data in the common map.
func replaceAllSources(tmpl interface{}, src []discovery.Discovery, common data.Map, rc replaceConfig) (transformedData []data.Transformed) {
	templateVal := reflect.ValueOf(tmpl)
	for _, discov := range src {
		matches := 0
		replaced := replaceFields([]data.Map{discov.Variables, common}, templateVal, rc, &matches)
		entityRewrites := replaceEntityRewrites([]data.Map{discov.Variables, common}, discov.EntityRewrites, rc)

		if matches == 0 { // the config has no variables. Returning single instance
			return []data.Transformed{{Variables: tmpl, EntityRewrites: entityRewrites}}
		}

		transformedData = append(transformedData,
			data.Transformed{
				Variables:         replaced.Interface(),
				MetricAnnotations: data.InterfaceMapToMap(discov.MetricAnnotations),
				EntityRewrites:    entityRewrites,
			})
	}
	return transformedData
}

func replaceEntityRewrites(values []data.Map, entityRewrite []data.EntityRewrite, rc replaceConfig) []data.EntityRewrite {

	for i := range entityRewrite {
		entityRewrite[i].ReplaceField = naming.AddPrefixToVariable(data.DiscoveryPrefix, entityRewrite[i].ReplaceField)
		entityRewrite[i].Match = naming.AddPrefixToVariable(data.DiscoveryPrefix, entityRewrite[i].Match)
	}

	entityRewriteTpl := reflect.ValueOf(entityRewrite)

	entityRewriteMatches := 0

	entityRewriteReplaced := replaceFields(values, entityRewriteTpl, rc, &entityRewriteMatches)

	if entityRewriteMatches > 0 {
		entityRewrite = entityRewriteReplaced.Interface().([]data.EntityRewrite)
	}
	return entityRewrite
}

func replaceAllBytes(template []byte, src []discovery.Discovery, common data.Map, rc replaceConfig) [][]byte {
	var allReplaced [][]byte
	for _, discov := range src {
		matches := 0
		replaced := replaceBytes([]data.Map{discov.Variables, common}, template, rc, &matches)
		if matches == 0 { // the config has no variables. Returning single instance
			return [][]byte{template}
		}
		allReplaced = append(allReplaced, replaced)
	}
	return allReplaced
}

func replaceFields(values []data.Map, val reflect.Value, rc replaceConfig, matches *int) reflect.Value {
	switch val.Kind() {
	case reflect.Slice:
		// return provided slice if is empty
		if val.Len() == 0 {
			return val
		}

		// if it is a byte array, replaces it as if it were a string
		if val.Type().Elem().Kind() == reflect.Uint8 {
			replaced := replaceBytes(values, val.Bytes(), rc, matches)
			return reflect.ValueOf(replaced)
		}
		length := val.Len()
		newSlice := reflect.MakeSlice(val.Type(), length, length)
		for i := 0; i < length; i++ {
			ival := val.Index(i)
			replaced := replaceFields(values, ival, rc, matches)
			newSlice.Index(i).Set(replaced)
		}
		return newSlice
	case reflect.Ptr:
		if val.IsNil() {
			return val.Elem()
		}
		vals := replaceFields(values, val.Elem(), rc, matches)
		if vals.Kind() == reflect.Ptr {
			return reflect.NewAt(val.Type(), unsafe.Pointer(vals.Pointer()))
		}
		if vals.CanAddr() {
			return vals.Addr()
		}
		return val.Elem()
	case reflect.Interface:
		vals := replaceFields(values, reflect.ValueOf(val.Interface()), rc, matches)
		if vals.Kind() == reflect.Ptr {
			return reflect.NewAt(val.Type(), unsafe.Pointer(vals.Pointer()))
		}
		return vals
	case reflect.String:
		nStr := replaceBytes(values, []byte(val.String()), rc, matches)
		return reflect.ValueOf(string(nStr))
	case reflect.Map:
		keys := val.MapKeys()
		if len(keys) == 0 {
			return val
		}

		newMap := reflect.MakeMap(val.Type())
		for _, k := range keys {
			val := val.MapIndex(k)
			nComps := replaceFields(values, val, rc, matches)
			newMap.SetMapIndex(k, nComps)
		}
		return newMap
	case reflect.Struct:
		newStruct := reflect.New(val.Type()).Elem()
		for i := 0; i < val.NumField(); i++ {
			nComps := replaceFields(values, val.Field(i), rc, matches)
			field := newStruct.Field(i)
			if field.CanSet() && nComps.IsValid() {
				field.Set(nComps)
			}
		}
		return newStruct
	default:
		return val
	}
}

func replaceBytes(values []data.Map, template []byte, rc replaceConfig, nMatches *int) []byte {
	matches := regex.FindAllIndex(template, -1)
	if len(matches) == 0 {
		return template // zero variables, all have been found and replaced
	}
	replace := make([]byte, 0, len(template))
	replace = append(replace, template[:matches[0][0]]...)
	for i := 0; i < len(matches)-1; i++ {
		value := variable(values, template[matches[i][0]:matches[i][1]], rc)
		*nMatches++
		replace = append(replace, value...)
		replace = append(replace, template[matches[i][1]:matches[i+1][0]]...)
	}
	last := len(matches) - 1
	value := variable(values, template[matches[last][0]:matches[last][1]], rc)
	*nMatches++
	replace = append(replace, value...)
	replace = append(replace, template[matches[last][1]:]...)
	return replace
}

// replaces a variable mark from its corresponding variable or discovered item.
func variable(values []data.Map, match []byte, rc replaceConfig) []byte {
	// removing ${...}
	varName := string(bytes.Trim(match, "${}\n\r\t "))

	for _, vmap := range values {
		if value, ok := vmap[varName]; ok {
			return []byte(value)
		}
	}

	// if not found in the discovered/variables static sources, we ask dynamically for it
	for _, onDemand := range rc.onDemand {
		if value, ok := onDemand(varName); ok {
			return value
		}
	}

	// if the value is not found, returns the match itself
	return match
}
