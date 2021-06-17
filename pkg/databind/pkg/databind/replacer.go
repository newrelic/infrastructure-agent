// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"bytes"
	"errors"
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
			// TRICKY LOGIC HERE
			// if no discovery nor variables, we use this invocation not to replace anything, but
			// to check if there are variable placeholders in the template (observe that we are passing
			// an empty discovery source in the second argument)
			_, err := replaceAllSources(template, []discovery.Discovery{{}}, data.Map{}, rc)
			// if the above returned error, it means it has variables. So since discovery returned
			// no results, we will to return an empty array
			if err != nil {
				return transformedData, nil
			}
			// otherwise, it means it does not have variables, so we will return the template as it was
			// since it was not bounded to any discovery process
			return []data.Transformed{{Variables: template}}, nil
		}
		// if no discovery data but variables, we just replace variables as if they were
		// a discovery source and leave the "common" values as empty
		return replaceAllSources(template, []discovery.Discovery{{Variables: vals.vars}}, data.Map{}, rc)
	}

	discoverySources := vals.discov
	varSrc := vals.vars

	return replaceAllSources(template, discoverySources, varSrc, rc)
}

// ReplaceBytes receives a byte array that may  contain ${variable} placeholders,
// and returns an array of byte arrays replacing the variable placeholders from the respective Values.
func ReplaceBytes(vals *Values, template []byte, options ...ReplaceOption) ([][]byte, error) {
	rc := replaceConfig{}
	for _, option := range options {
		option(&rc)
	}
	if len(vals.discov) == 0 {
		if len(vals.vars) == 0 {
			// the same tricky logic as for "Replace" function
			_, err := replaceAllBytes(template, []discovery.Discovery{{}}, data.Map{}, rc)
			if err != nil {
				return [][]byte{}, nil
			}
			return [][]byte{template}, nil
		}
		// if no discovery data but variables, we just replace variables as if they were
		// a discovery source and leave the "common" values as empty
		return replaceAllBytes(template, []discovery.Discovery{{Variables: vals.vars}}, data.Map{}, rc)
	}

	discoverySources := vals.discov
	varSrc := vals.vars

	return replaceAllBytes(template, discoverySources, varSrc, rc)
}

// Replaces all the discovery sources by the values in the "src" array. The common array is shared
// If src is empty, no change is done even if there is data in the common map.
func replaceAllSources(tmpl interface{}, src []discovery.Discovery, common data.Map, rc replaceConfig) (transformedData []data.Transformed, err error) {
	templateVal := reflect.ValueOf(tmpl)
	for _, discov := range src {
		matches := 0
		replaced, err := replaceFields([]data.Map{discov.Variables, common}, templateVal, rc, &matches)
		if err != nil {
			return transformedData, err
		}

		entityRewrites, err := replaceEntityRewrites([]data.Map{discov.Variables, common}, discov.EntityRewrites, rc)
		if err != nil {
			return transformedData, err
		}

		if matches == 0 { // the config has no variables. Returning single instance
			return []data.Transformed{{Variables: tmpl, EntityRewrites: entityRewrites}}, nil
		}

		transformedData = append(transformedData,
			data.Transformed{
				Variables:         replaced.Interface(),
				MetricAnnotations: data.InterfaceMapToMap(discov.MetricAnnotations),
				EntityRewrites:    entityRewrites,
			})
	}
	return transformedData, nil
}

func replaceEntityRewrites(values []data.Map, entityRewrite []data.EntityRewrite, rc replaceConfig) ([]data.EntityRewrite, error) {

	for i := range entityRewrite {
		entityRewrite[i].ReplaceField = naming.AddPrefixToVariable(data.DiscoveryPrefix, entityRewrite[i].ReplaceField)
		entityRewrite[i].Match = naming.AddPrefixToVariable(data.DiscoveryPrefix, entityRewrite[i].Match)
	}

	entityRewriteTpl := reflect.ValueOf(entityRewrite)

	entityRewriteMatches := 0

	entityRewriteReplaced, err := replaceFields(values, entityRewriteTpl, rc, &entityRewriteMatches)
	if err != nil {
		return nil, err
	}

	if entityRewriteMatches > 0 {
		entityRewrite = entityRewriteReplaced.Interface().([]data.EntityRewrite)
	}
	return entityRewrite, nil
}

func replaceAllBytes(template []byte, src []discovery.Discovery, common data.Map, rc replaceConfig) ([][]byte, error) {
	var allReplaced [][]byte
	for _, discov := range src {
		matches := 0
		replaced, err := replaceBytes([]data.Map{discov.Variables, common}, template, rc, &matches)
		if err != nil {
			return nil, err
		}
		if matches == 0 { // the config has no variables. Returning single instance
			return [][]byte{template}, nil
		}
		allReplaced = append(allReplaced, replaced)
	}
	return allReplaced, nil
}

func replaceFields(values []data.Map, val reflect.Value, rc replaceConfig, matches *int) (reflect.Value, error) {
	switch val.Kind() {
	case reflect.Slice:
		// if it is a byte array, replaces it as if it were a string
		if val.Type().Elem().Kind() == reflect.Uint8 {
			replaced, err := replaceBytes(values, val.Bytes(), rc, matches)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(replaced), nil
		}
		length := val.Len()
		newSlice := reflect.MakeSlice(val.Type(), length, length)
		for i := 0; i < length; i++ {
			ival := val.Index(i)
			replaced, err := replaceFields(values, ival, rc, matches)
			if err != nil {
				return reflect.Value{}, err
			}
			newSlice.Index(i).Set(replaced)
		}
		return newSlice, nil
	case reflect.Ptr:
		if val.IsNil() {
			return val.Elem(), nil
		}
		vals, err := replaceFields(values, val.Elem(), rc, matches)
		if err != nil {
			return reflect.Value{}, err
		}
		if vals.Kind() == reflect.Ptr {
			return reflect.NewAt(val.Type(), unsafe.Pointer(vals.Pointer())), nil
		}
		if vals.CanAddr() {
			return vals.Addr(), nil
		}
		return val.Elem(), nil
	case reflect.Interface:
		vals, err := replaceFields(values, reflect.ValueOf(val.Interface()), rc, matches)
		if err != nil {
			return reflect.Value{}, err
		}
		if vals.Kind() == reflect.Ptr {
			return reflect.NewAt(val.Type(), unsafe.Pointer(vals.Pointer())), nil
		}
		return vals, nil
	case reflect.String:
		nStr, err := replaceBytes(values, []byte(val.String()), rc, matches)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(string(nStr)), nil
	case reflect.Map:
		newMap := reflect.MakeMap(val.Type())
		keys := val.MapKeys()
		for _, k := range keys {
			val := val.MapIndex(k)
			nComps, err := replaceFields(values, val, rc, matches)
			if err != nil {
				return reflect.Value{}, err
			}
			newMap.SetMapIndex(k, nComps)
		}
		return newMap, nil
	case reflect.Struct:
		newStruct := reflect.New(val.Type()).Elem()
		for i := 0; i < val.NumField(); i++ {
			nComps, err := replaceFields(values, val.Field(i), rc, matches)
			if err != nil {
				return reflect.Value{}, err
			}
			field := newStruct.Field(i)
			if field.CanSet() && nComps.IsValid() {
				field.Set(nComps)
			}
		}
		return newStruct, nil
	default:
		return val, nil
	}
}

func replaceBytes(values []data.Map, template []byte, rc replaceConfig, nMatches *int) ([]byte, error) {
	matches := regex.FindAllIndex(template, -1)
	if len(matches) == 0 {
		return template, nil // zero variables, all have been found and replaced
	}
	replace := make([]byte, 0, len(template))
	replace = append(replace, template[:matches[0][0]]...)
	for i := 0; i < len(matches)-1; i++ {
		value, err := variable(values, template[matches[i][0]:matches[i][1]], rc)
		if err != nil {
			return nil, err
		}
		*nMatches++
		replace = append(replace, value...)
		replace = append(replace, template[matches[i][1]:matches[i+1][0]]...)
	}
	last := len(matches) - 1
	value, err := variable(values, template[matches[last][0]:matches[last][1]], rc)
	if err != nil {
		return nil, err
	}
	*nMatches++
	replace = append(replace, value...)
	replace = append(replace, template[matches[last][1]:]...)
	return replace, err
}

// replaces a variable mark from its corresponding variable or discovered item.
func variable(values []data.Map, match []byte, rc replaceConfig) ([]byte, error) {
	// removing ${...}
	varName := string(bytes.Trim(match, "${}\n\r\t "))

	for _, vmap := range values {
		if value, ok := vmap[varName]; ok {
			return []byte(value), nil
		}
	}

	// if not found in the discovered/variables static sources, we ask dynamically for it
	for _, onDemand := range rc.onDemand {
		if value, ok := onDemand(varName); ok {
			return value, nil
		}
	}

	// if the value is not found, returns the match itself
	return match, errors.New("value not found: " + varName)
}
