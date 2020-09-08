// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func Example() {
	h, err := NewHarvester(
		// APIKey is the only required field and refers to your New Relic Insights Insert API key.
		ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")),
		ConfigCommonAttributes(map[string]interface{}{
			"app.name": "myApplication",
		}),
		ConfigBasicErrorLogger(os.Stderr),
		ConfigBasicDebugLogger(os.Stdout),
	)
	if err != nil {
		fmt.Println(err)
	}

	// Record Gauge, Count, and Summary metrics using RecordMetric. These
	// metrics are not aggregated.  This is useful for exporting metrics
	// recorded by another system.
	h.RecordMetric(Gauge{
		Timestamp: time.Now(),
		Value:     1,
		Name:      "myMetric",
		Attributes: map[string]interface{}{
			"color": "purple",
		},
	})

	// Record spans using RecordSpan.
	h.RecordSpan(Span{
		ID:          "12345",
		TraceID:     "67890",
		Name:        "purple-span",
		Timestamp:   time.Now(),
		Duration:    time.Second,
		ServiceName: "ExampleApplication",
		Attributes: map[string]interface{}{
			"color": "purple",
		},
	})

	// Aggregate individual datapoints into metrics using the
	// MetricAggregator.  You can do this in a single line:
	h.MetricAggregator().Count("myCounter", map[string]interface{}{"color": "pink"}).Increment()
	// Or keep a metric reference for fast accumulation:
	counter := h.MetricAggregator().Count("myCounter", map[string]interface{}{"color": "pink"})
	for i := 0; i < 100; i++ {
		counter.Increment()
	}
}

func ExampleNewHarvester() {
	h, err := NewHarvester(
		ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")),
	)
	if err != nil {
		fmt.Println(err)
	}
	h.RecordMetric(Gauge{
		Timestamp: time.Now(),
		Value:     1,
		Name:      "myMetric",
		Attributes: map[string]interface{}{
			"color": "purple",
		},
	})
}

func ExampleHarvester_RecordMetric() {
	h, _ := NewHarvester(
		ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")),
	)
	start := time.Now()
	h.RecordMetric(Count{
		Name:           "myCount",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          123,
		Timestamp:      start,
		Interval:       5 * time.Second,
	})
}

func ExampleConfigSpansURLOverride() {
	h, _ := NewHarvester(
		ConfigAPIKey(os.Getenv("NEW_RELIC_INSIGHTS_INSERT_API_KEY")),
		// Use ConfigSpansURLOverride to enable Infinite Tracing on the New
		// Relic Edge by passing it your Trace Observer URL, including scheme
		// and path.
		ConfigSpansURLOverride("https://nr-internal.aws-us-east-1.tracing.edge.nr-data.net/trace/v1"),
	)
	h.RecordSpan(Span{
		ID:          "12345",
		TraceID:     "67890",
		Name:        "purple-span",
		Timestamp:   time.Now(),
		Duration:    time.Second,
		ServiceName: "ExampleApplication",
		Attributes: map[string]interface{}{
			"color": "purple",
		},
	})
}
