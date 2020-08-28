// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cumulative

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	telemetry "github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi"
)

func Example() {
	h, err := telemetry.NewHarvester(
		telemetry.ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")),
	)
	if err != nil {
		fmt.Println(err)
	}
	dc := NewDeltaCalculator()

	attributes := map[string]interface{}{
		"id":  123,
		"zip": "zap",
	}
	for {
		cumulativeValue := float64(time.Now().Unix())
		if m, ok := dc.CountMetric("secondsElapsed", attributes, cumulativeValue, time.Now()); ok {
			h.RecordMetric(m)
		}
		time.Sleep(5 * time.Second)
	}
}

func TestCountMetricBasicUse(t *testing.T) {
	// Test expected usage of CountMetric.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	dc := NewDeltaCalculator()
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 100.0, now); ok {
		t.Error(ok)
	}
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": 1}, 200.0, now); ok {
		t.Error(ok)
	}
	if _, ok := dc.CountMetric("m2", map[string]interface{}{"zip": "zap"}, 300.0, now); ok {
		t.Error(ok)
	}
	m, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 105.0, now.Add(1*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:           "m1",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          5.0,
		Timestamp:      now,
		Interval:       1 * time.Minute,
	}) {
		t.Error(ok, m)
	}
	m, ok = dc.CountMetric("m1", map[string]interface{}{"zip": 1}, 206.0, now.Add(1*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:           "m1",
		AttributesJSON: json.RawMessage(`{"zip":1}`),
		Value:          6.0,
		Timestamp:      now,
		Interval:       1 * time.Minute,
	}) {
		t.Error(ok, m)
	}
	m, ok = dc.CountMetric("m2", map[string]interface{}{"zip": "zap"}, 307.0, now.Add(1*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:           "m2",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          7.0,
		Timestamp:      now,
		Interval:       1 * time.Minute,
	}) {
		t.Error(ok, m)
	}
}

func TestCountZeroDelta(t *testing.T) {
	// Test that adding the same value twice results in a Count with a zero
	// value.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	ats := map[string]interface{}{"zip": "zap"}
	dc := NewDeltaCalculator()
	if _, ok := dc.CountMetric("m1", ats, 5.0, now); ok {
		t.Error(ok)
	}
	m, ok := dc.CountMetric("m1", ats, 5.0, now.Add(1*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:           "m1",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          0.0,
		Timestamp:      now,
		Interval:       1 * time.Minute,
	}) {
		t.Error(ok, m)
	}
}

func TestCountMetricNegativeDeltaReset(t *testing.T) {
	// Test that CountMetric does not return a count metric with a negative
	// value.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	dc := NewDeltaCalculator()
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 5.0, now); ok {
		t.Error(ok)
	}
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 4.0, now.Add(1*time.Minute)); ok {
		t.Error(ok)
	}
	m, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 7.0, now.Add(2*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:           "m1",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          3.0,
		Timestamp:      now.Add(1 * time.Minute),
		Interval:       1 * time.Minute,
	}) {
		t.Error(ok, m)
	}
}

func TestTimestampOrder(t *testing.T) {
	// Test that CountMetric does not return a count metric when the
	// timestamp values are not in increasing order.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	dc := NewDeltaCalculator()
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 5.0, now); ok {
		t.Error(ok)
	}
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 6.0, now.Add(-1*time.Minute)); ok {
		t.Error(ok)
	}
	m, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 7.0, now.Add(1*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:           "m1",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          2.0,
		Timestamp:      now,
		Interval:       1 * time.Minute,
	}) {
		t.Error(ok, m)
	}
}

func TestCountMetricNoAttributes(t *testing.T) {
	// Test that CountMetric works when no attributes are provided.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	dc := NewDeltaCalculator()
	if _, ok := dc.CountMetric("m1", nil, 5.0, now); ok {
		t.Error(ok)
	}
	m, ok := dc.CountMetric("m1", nil, 10.0, now.Add(1*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:      "m1",
		Value:     5.0,
		Timestamp: now,
		Interval:  1 * time.Minute,
	}) {
		t.Error(ok, m)
	}
}

func TestExpirationDefaults(t *testing.T) {
	// Test that expiration happens with the default settings.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	dc := NewDeltaCalculator()
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 5.0, now); ok {
		t.Error(ok)
	}
	if _, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 10.0, now.Add(21*time.Minute)); ok {
		t.Error(ok)
	}
	m, ok := dc.CountMetric("m1", map[string]interface{}{"zip": "zap"}, 12.0, now.Add(40*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:           "m1",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          2.0,
		Timestamp:      now.Add(21 * time.Minute),
		Interval:       19 * time.Minute,
	}) {
		t.Error(ok, m)
	}
}

func TestExpirationCustomSettings(t *testing.T) {
	// Test that expiration happens with custom settings.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	dc := NewDeltaCalculator().
		SetExpirationAge(5 * time.Minute).
		SetExpirationCheckInterval(10 * time.Minute)

	if _, ok := dc.CountMetric("m1", nil, 5.0, now); ok {
		t.Error(ok)
	}
	if _, ok := dc.CountMetric("m2", nil, 5.0, now.Add(9*time.Minute)); ok {
		t.Error(ok)
	}
	if _, ok := dc.CountMetric("m1", nil, 10.0, now.Add(11*time.Minute)); ok {
		t.Error(ok)
	}
	m, ok := dc.CountMetric("m2", nil, 10.0, now.Add(11*time.Minute))
	if !ok || !reflect.DeepEqual(m, telemetry.Count{
		Name:      "m2",
		Value:     5.0,
		Timestamp: now.Add(9 * time.Minute),
		Interval:  2 * time.Minute,
	}) {
		t.Error(ok, m)
	}
}

func TestManyAttributes(t *testing.T) {
	// Test that attributes are turned into JSON in a fixed order.  Note
	// that if JSON attribute order is random this test may still
	// occasionally pass.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	dc := NewDeltaCalculator()

	attributes := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		attributes[strconv.Itoa(i)] = i
	}
	dc.CountMetric("myMetric", attributes, 5.0, now)
	_, ok := dc.CountMetric("myMetric", attributes, 6.0, now.Add(1*time.Minute))
	if !ok {
		t.Error(ok)
	}
}
