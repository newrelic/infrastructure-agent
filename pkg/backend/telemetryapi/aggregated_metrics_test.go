// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"context"
	"math"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestDifferentAttributes(t *testing.T) {
	// Test that attributes contribute to identity, ie, metrics with the
	// same name but different attributes should generate different metrics.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	h, _ := NewHarvester(configTesting)
	h.MetricAggregator().Gauge("myGauge", map[string]interface{}{"zip": "zap"}).valueNow(1.0, now)
	h.MetricAggregator().Gauge("myGauge", map[string]interface{}{"zip": "zup"}).valueNow(2.0, now)
	expect := `[
		{"name":"myGauge","type":"gauge","value":1,"timestamp":1417136460000,"attributes":{"zip":"zap"}},
		{"name":"myGauge","type":"gauge","value":2,"timestamp":1417136460000,"attributes":{"zip":"zup"}}
	]`
	testHarvesterMetrics(t, h, expect)
}

func TestSameNameDifferentTypes(t *testing.T) {
	// Test that type contributes to identity, ie, metrics with the same
	// name and same attributes of different types should generate different
	// metrics.
	h, _ := NewHarvester(configTesting)
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	h.MetricAggregator().Gauge("metric", map[string]interface{}{"zip": "zap"}).valueNow(1.0, now)
	h.MetricAggregator().Count("metric", map[string]interface{}{"zip": "zap"}).Increment()
	h.MetricAggregator().Summary("metric", map[string]interface{}{"zip": "zap"}).Record(1.0)
	expect := `[
		{"name":"metric","type":"count","value":1,"attributes":{"zip":"zap"}},
		{"name":"metric","type":"gauge","value":1,"timestamp":1417136460000,"attributes":{"zip":"zap"}},
		{"name":"metric","type":"summary","value":{"sum":1,"count":1,"min":1,"max":1},"attributes":{"zip":"zap"}}
	]`
	testHarvesterMetrics(t, h, expect)
}

func TestManyAttributes(t *testing.T) {
	// Test adding the same metric with many attributes to ensure that
	// attributes are serialized into JSON in a fixed order.  Note that if
	// JSON attribute order is random this test may still occasionally pass.
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	h, _ := NewHarvester(configTesting)
	attributes := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		attributes[strconv.Itoa(i)] = i
	}
	h.MetricAggregator().Gauge("myGauge", attributes).valueNow(1.0, now)
	h.MetricAggregator().Gauge("myGauge", attributes).valueNow(2.0, now)
	if ms := h.swapOutMetrics(context.Background(), time.Now()); len(ms) != 1 {
		t.Fatal(len(ms))
	}
}

func BenchmarkAggregatedMetric(b *testing.B) {
	// This benchmark tests creating and aggregating a summary.
	h, _ := NewHarvester(configTesting)
	attributes := map[string]interface{}{"zip": "zap", "zop": 123}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		summary := h.MetricAggregator().Summary("mySummary", attributes)
		summary.Record(12.3)
		if nil == summary {
			b.Fatal("nil summary")
		}
	}
}

func ExampleMetricAggregator_Count() {
	h, _ := NewHarvester(ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")))
	count := h.MetricAggregator().Count("myCount", map[string]interface{}{"zip": "zap"})
	count.Increment()
}

func ExampleMetricAggregator_Gauge() {
	h, _ := NewHarvester(ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")))
	gauge := h.MetricAggregator().Gauge("temperature", map[string]interface{}{"zip": "zap"})
	gauge.Value(23.4)
}

func ExampleMetricAggregator_Summary() {
	h, _ := NewHarvester(ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")))
	summary := h.MetricAggregator().Summary("mySummary", map[string]interface{}{"zip": "zap"})
	summary.RecordDuration(3 * time.Second)
}

func TestGauge(t *testing.T) {
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	h, _ := NewHarvester(configTesting)
	h.MetricAggregator().Gauge("myGauge", map[string]interface{}{"zip": "zap"}).valueNow(123.4, now)

	expect := `[{"name":"myGauge","type":"gauge","value":123.4,"timestamp":1417136460000,"attributes":{"zip":"zap"}}]`
	testHarvesterMetrics(t, h, expect)
}

func TestNilAggregatorGauges(t *testing.T) {
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	var h *Harvester
	gauge := h.MetricAggregator().Gauge("gauge", map[string]interface{}{})
	gauge.valueNow(5.5, now)
}

func TestNilGaugeMethods(t *testing.T) {
	now := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	var gauge *AggregatedGauge
	gauge.valueNow(5.5, now)
}

func TestGaugeNilAggregator(t *testing.T) {
	g := AggregatedGauge{}
	g.Value(10)
}

func TestCount(t *testing.T) {
	h, _ := NewHarvester(configTesting)
	count := h.MetricAggregator().Count("myCount", map[string]interface{}{"zip": "zap"})
	count.Increase(22.5)
	count.Increment()

	expect := `[{"name":"myCount","type":"count","value":23.5,"attributes":{"zip":"zap"}}]`
	testHarvesterMetrics(t, h, expect)
}

func TestCountNegative(t *testing.T) {
	h, _ := NewHarvester(configTesting)
	count := h.MetricAggregator().Count("myCount", map[string]interface{}{"zip": "zap"})
	count.Increase(-123)
	if ms := h.swapOutMetrics(context.Background(), time.Now()); len(ms) != 0 {
		t.Fatal(ms)
	}
}

func TestNilAggregatorCounts(t *testing.T) {
	var h *Harvester
	count := h.MetricAggregator().Count("count", map[string]interface{}{})
	count.Increment()
	count.Increase(5)
}

func TestNilCountMethods(t *testing.T) {
	var count *AggregatedCount
	count.Increment()
	count.Increase(5)
}

func TestCountNilAggregator(t *testing.T) {
	c := AggregatedCount{}
	c.Increment()
	c.Increase(1)
}

func TestSummary(t *testing.T) {
	h, _ := NewHarvester(configTesting)
	summary := h.MetricAggregator().Summary("mySummary", map[string]interface{}{"zip": "zap"})
	summary.Record(3)
	summary.Record(4)
	summary.Record(5)

	expect := `[{"name":"mySummary","type":"summary","value":{"sum":12,"count":3,"min":3,"max":5},"attributes":{"zip":"zap"}}]`
	testHarvesterMetrics(t, h, expect)
}

func TestSummaryDuration(t *testing.T) {
	h, _ := NewHarvester(configTesting)
	summary := h.MetricAggregator().Summary("mySummary", map[string]interface{}{"zip": "zap"})
	summary.RecordDuration(3 * time.Second)
	summary.RecordDuration(4 * time.Second)
	summary.RecordDuration(5 * time.Second)

	expect := `[{"name":"mySummary","type":"summary","value":{"sum":12000,"count":3,"min":3000,"max":5000},"attributes":{"zip":"zap"}}]`
	testHarvesterMetrics(t, h, expect)
}

func TestNilAggregatorSummaries(t *testing.T) {
	var h *Harvester
	summary := h.MetricAggregator().Summary("summary", map[string]interface{}{})
	summary.Record(1)
	summary.RecordDuration(time.Second)
}

func TestNilSummaryMethods(t *testing.T) {
	var summary *AggregatedSummary
	summary.Record(1)
	summary.RecordDuration(time.Second)
}

func TestSummaryNilAggregator(t *testing.T) {
	s := AggregatedSummary{}
	s.Record(10)
	s.RecordDuration(time.Second)
}

func TestSummaryMinMax(t *testing.T) {
	h, _ := NewHarvester(configTesting)
	s := h.MetricAggregator().Summary("sum", nil)
	s.Record(2)
	s.Record(1)
	s.Record(3)
	expect := `[{"name":"sum","type":"summary","value":{"sum":6,"count":3,"min":1,"max":3},"attributes":{}}]`
	testHarvesterMetrics(t, h, expect)
}

func configSaveErrors(savedErrors *[]map[string]interface{}) func(cfg *Config) {
	return func(cfg *Config) {
		cfg.ErrorLogger = func(e map[string]interface{}) {
			*savedErrors = append(*savedErrors, e)
		}
	}
}

func TestInvalidAggregatedSummaryValue(t *testing.T) {
	var summary *AggregatedSummary
	summary.Record(math.NaN())

	var savedErrors []map[string]interface{}
	h, _ := NewHarvester(configTesting, configSaveErrors(&savedErrors))
	summary = h.MetricAggregator().Summary("summary", map[string]interface{}{})
	summary.Record(math.NaN())

	if len(savedErrors) != 1 || !reflect.DeepEqual(savedErrors[0], map[string]interface{}{
		"err":     errFloatNaN.Error(),
		"message": "invalid aggregated summary value",
	}) {
		t.Error(savedErrors)
	}
	if len(h.aggregatedMetrics) != 0 {
		t.Error(h.aggregatedMetrics)
	}
}

func TestInvalidAggregatedCountValue(t *testing.T) {
	var count *AggregatedCount
	count.Increase(math.Inf(1))

	var savedErrors []map[string]interface{}
	h, _ := NewHarvester(configTesting, configSaveErrors(&savedErrors))
	count = h.MetricAggregator().Count("count", map[string]interface{}{})
	count.Increase(math.Inf(1))

	if len(savedErrors) != 1 || !reflect.DeepEqual(savedErrors[0], map[string]interface{}{
		"err":     errFloatInfinity.Error(),
		"message": "invalid aggregated count value",
	}) {
		t.Error(savedErrors)
	}
	if len(h.aggregatedMetrics) != 0 {
		t.Error(h.aggregatedMetrics)
	}
}

func TestInvalidAggregatedGaugeValue(t *testing.T) {
	var gauge *AggregatedGauge
	gauge.Value(math.Inf(-1))

	var savedErrors []map[string]interface{}
	h, _ := NewHarvester(configTesting, configSaveErrors(&savedErrors))
	gauge = h.MetricAggregator().Gauge("gauge", map[string]interface{}{})
	gauge.Value(math.Inf(-1))

	if len(savedErrors) != 1 || !reflect.DeepEqual(savedErrors[0], map[string]interface{}{
		"err":     errFloatInfinity.Error(),
		"message": "invalid aggregated gauge value",
	}) {
		t.Error(savedErrors)
	}
	if len(h.aggregatedMetrics) != 0 {
		t.Error(h.aggregatedMetrics)
	}
}
