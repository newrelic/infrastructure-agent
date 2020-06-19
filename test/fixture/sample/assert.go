// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/metrics"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/test/proxy/minagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertRequestContainsSample asserts that the given request contains the given sample values.
// It does not count the amount of samples.
func AssertRequestContainsSample(t *testing.T, req http.Request, expected sample.Event) {
	reqBytes, err := ioutil.ReadAll(req.Body)
	assert.NoError(t, err)

	sentBatch := agent.MetricPostBatch{}
	assert.NoError(t, json.Unmarshal(reqBytes, &sentBatch))

	require.Len(t, sentBatch, 1)
	got := sentBatch[0]

	for _, gotEv := range got.Events {

		switch expected.(type) {

		case *network.NetworkSample:
			var gotSample network.NetworkSample
			assert.NoError(t, json.Unmarshal(gotEv, &gotSample))
			assert.NotNil(t, expected)
			assert.NotNil(t, gotSample)
			assert.EqualValues(t, expected.(*network.NetworkSample).EntityKey, gotSample.EntityKey)
			expected.Timestamp(0)
			gotSample.Timestamp(0)
			assert.Equal(t, expected, &gotSample)

		case *metrics.ProcessSample:
			var gotSample metrics.ProcessSample
			assert.NoError(t, json.Unmarshal(gotEv, &gotSample))
			assert.NotNil(t, expected)
			assert.NotNil(t, gotSample)
			expected.Timestamp(0)
			gotSample.Timestamp(0)
			assert.Equal(t, expected, &gotSample)
			assert.EqualValues(t, expected.(*metrics.ProcessSample).EntityKey, gotSample.EntityKey)

		case *storage.Sample:
			var gotSample storage.Sample
			assert.NoError(t, json.Unmarshal(gotEv, &gotSample))
			assert.NotNil(t, expected)
			assert.NotNil(t, gotSample)
			expected.Timestamp(0)
			gotSample.Timestamp(0)
			assert.Equal(t, expected, &gotSample)
			assert.EqualValues(t, expected.(*storage.Sample).EntityKey, gotSample.EntityKey)

		case *metrics.SystemSample:
			var gotSample metrics.SystemSample
			assert.NoError(t, json.Unmarshal(gotEv, &gotSample))

			expected.Timestamp(0)
			gotSample.Timestamp(0)

			// create expectations
			expectedSample := reflect.ValueOf(expected).Elem()
			expectValues := make([]interface{}, expectedSample.NumField())
			//expectFields := make([]string, expectedSample.NumField())
			for i := 0; i < expectedSample.NumField(); i++ {
				//expectFields[i] = expectedSample.Type().Field(i).Name
				expectValues[i] = expectedSample.Field(i).Interface()
			}

			// assert every expectation for that sample
			for _, v := range expectValues {

				// atm we don't assert on this
				if _, ok := v.(sample.BaseEvent); ok {
					continue
				}

				// atm we don't assert on empty values
				if isEmpty(v) {
					continue
				}

				//t.Logf("assert on Sample: %+v", v)
				switch v.(type) {

				case *metrics.CPUSample:
					assert.Equal(t, *v.(*metrics.CPUSample), *gotSample.CPUSample)

				case *metrics.LoadSample:
					assert.Equal(t, *v.(*metrics.LoadSample), *gotSample.LoadSample)

				case *metrics.MemorySample:
					assert.Equal(t, *v.(*metrics.MemorySample), *gotSample.MemorySample)

				case *metrics.DiskSample:
					assert.Equal(t, *v.(*metrics.DiskSample), *gotSample.DiskSample)

				default:
					t.Errorf("unexpected value format for %+v", v)
				}
			}
			assert.EqualValues(t, expected.(*metrics.SystemSample).EntityKey, gotSample.EntityKey)
		case minagent.FakeSample:
			var gotSample minagent.FakeSample
			assert.NoError(t, json.Unmarshal(gotEv, &gotSample))
			assert.NotNil(t, expected)
			assert.NotNil(t, gotSample)
			expected.Timestamp(0)
			gotSample.Timestamp(0)
			assert.Equal(t, expected, gotSample)
		}
	}
}

// isEmpty gets whether the specified object is considered empty or not.
// Ported from testify.
func isEmpty(object interface{}) bool {

	// get nil case out of the way
	if object == nil {
		return true
	}

	objValue := reflect.ValueOf(object)

	switch objValue.Kind() {
	// collection types are empty when they have no element
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		return objValue.Len() == 0
		// pointers are empty if nil or if the value they point to is empty
	case reflect.Ptr:
		if objValue.IsNil() {
			return true
		}
		deref := objValue.Elem().Interface()
		return isEmpty(deref)
		// for all other types, compare against the zero value
	default:
		zero := reflect.Zero(objValue.Type())
		return reflect.DeepEqual(object, zero.Interface())
	}
}
