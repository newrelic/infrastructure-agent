// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestMetricPayload(t *testing.T) {
	// Test that a metric payload with timestamp, duration, and common
	// attributes correctly marshals into JSON.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	h, _ := NewHarvester(ConfigCommonAttributes(map[string]interface{}{"zop": "zup"}), configTesting)
	h.RecordMetric(Gauge{
		Name:       "metric",
		Attributes: map[string]interface{}{"zip": "zap"},
		Timestamp:  now,
		Value:      1.0,
	})
	h.RecordMetric(Summary{
		Name:       "summary-metric-nan-min",
		Attributes: map[string]interface{}{"zip": "zap"},
		Timestamp:  now,
		Count:      4.0,
		Sum:        1.0,
		Min:        math.NaN(),
		Max:        3.0,
	})
	h.RecordMetric(Summary{
		Name:       "summary-metric-nan-max",
		Attributes: map[string]interface{}{"zip": "zap"},
		Timestamp:  now,
		Count:      4.0,
		Sum:        1.0,
		Min:        10,
		Max:        math.NaN(),
	})
	h.lastHarvest = now
	end := h.lastHarvest.Add(5 * time.Second)
	reqs := h.swapOutMetrics(context.Background(), end)
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	js := reqs[0].UncompressedBody
	actual := string(js)
	expect := `[{
		"common":{
			"timestamp":1417136460000,
			"interval.ms":5000,
			"attributes":{"zop":"zup"}
		},
		"metrics":[
			{"name":"metric","type":"gauge","value":1,"timestamp":1417136460000,"attributes":{"zip":"zap"}},
			{"name":"summary-metric-nan-min","type":"summary","value":{"sum":1,"count":4,"min":null,"max":3},"timestamp":1417136460000,"attributes":{"zip":"zap"}},
			{"name":"summary-metric-nan-max","type":"summary","value":{"sum":1,"count":4,"min":10,"max":null},"timestamp":1417136460000,"attributes":{"zip":"zap"}}
		]
	}]`
	compactExpect := compactJSONString(expect)
	if compactExpect != actual {
		t.Errorf("\nexpect=%s\nactual=%s\n", compactExpect, actual)
	}
}

func TestVetAttributes(t *testing.T) {
	testcases := []struct {
		Input interface{}
		Valid bool
	}{
		// Valid attribute types.
		{Input: "string value", Valid: true},
		{Input: true, Valid: true},
		{Input: uint8(0), Valid: true},
		{Input: uint16(0), Valid: true},
		{Input: uint32(0), Valid: true},
		{Input: uint64(0), Valid: true},
		{Input: int8(0), Valid: true},
		{Input: int16(0), Valid: true},
		{Input: int32(0), Valid: true},
		{Input: int64(0), Valid: true},
		{Input: float32(0), Valid: true},
		{Input: float64(0), Valid: true},
		{Input: uint(0), Valid: true},
		{Input: int(0), Valid: true},
		{Input: uintptr(0), Valid: true},
		// Invalid attribute types.
		{Input: nil, Valid: false},
		{Input: struct{}{}, Valid: false},
		{Input: &struct{}{}, Valid: false},
		{Input: []int{1, 2, 3}, Valid: false},
	}

	for idx, tc := range testcases {
		key := "input"
		input := map[string]interface{}{
			key: tc.Input,
		}
		var errorLogged map[string]interface{}
		output := vetAttributes(input, func(e map[string]interface{}) {
			errorLogged = e
		})
		// Test the the input map has not been modified.
		if len(input) != 1 {
			t.Error("input map modified", input)
		}
		if tc.Valid {
			if len(output) != 1 {
				t.Error(idx, tc.Input, output)
			}
			if _, ok := output[key]; !ok {
				t.Error(idx, tc.Input, output)
			}
			if errorLogged != nil {
				t.Error(idx, "unexpected error present")
			}
		} else {
			if errorLogged == nil {
				t.Error(idx, "expected error missing")
			}
			if len(output) != 0 {
				t.Error(idx, tc.Input, output)
			}
		}
	}
}

func TestValidateCount(t *testing.T) {
	m := Count{
		Name:  "my-count",
		Value: math.NaN(),
	}
	if fields := m.validate(); !reflect.DeepEqual(fields, map[string]interface{}{
		"message": "invalid count value",
		"name":    "my-count",
		"err":     errFloatNaN.Error(),
	}) {
		t.Error(fields)
	}
	m = Count{
		Name:  "my-count",
		Value: 123.456,
	}
	if fields := m.validate(); fields != nil {
		t.Error(fields)
	}
}

func TestValidateGauge(t *testing.T) {
	m := Gauge{
		Name:  "my-gauge",
		Value: math.Inf(1),
	}
	if fields := m.validate(); !reflect.DeepEqual(fields, map[string]interface{}{
		"message": "invalid gauge field",
		"name":    "my-gauge",
		"err":     errFloatInfinity.Error(),
	}) {
		t.Error(fields)
	}
	m = Gauge{
		Name:  "my-gauge",
		Value: 123.456,
	}
	if fields := m.validate(); fields != nil {
		t.Error(fields)
	}
}

func TestValidateSummary(t *testing.T) {
	expectNaNErr := map[string]interface{}{
		"message": "invalid summary field",
		"name":    "my-summary",
		"err":     errFloatNaN.Error(),
	}
	expectInfErr := map[string]interface{}{
		"message": "invalid summary field",
		"name":    "my-summary",
		"err":     errFloatInfinity.Error(),
	}
	testcases := []struct {
		m      Summary
		fields map[string]interface{}
	}{
		{
			m:      Summary{Name: "my-summary", Count: 1.0, Sum: 2.0, Min: 3.0, Max: 4.0},
			fields: nil,
		},
		{
			m:      Summary{Name: "my-summary", Count: 1.0, Sum: 2.0, Min: math.NaN(), Max: math.NaN()},
			fields: nil,
		},
		{
			m:      Summary{Name: "my-summary", Count: math.NaN(), Sum: 2.0, Min: 3.0, Max: 4.0},
			fields: expectNaNErr,
		},
		{
			m:      Summary{Name: "my-summary", Count: 1.0, Sum: math.NaN(), Min: 3.0, Max: 4.0},
			fields: expectNaNErr,
		},
		{
			m:      Summary{Name: "my-summary", Count: 1.0, Sum: 2.0, Min: math.Inf(-3), Max: 4.0},
			fields: expectInfErr,
		},
		{
			m:      Summary{Name: "my-summary", Count: 1.0, Sum: 2.0, Min: 3.0, Max: math.Inf(3)},
			fields: expectInfErr,
		},
	}
	for idx, tc := range testcases {
		got := tc.m.validate()
		if !reflect.DeepEqual(got, tc.fields) {
			t.Error(idx, got, tc.fields)
		}
	}
}
