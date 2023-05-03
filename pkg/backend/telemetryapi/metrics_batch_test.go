// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi/internal"
)

func TestMetrics(t *testing.T) {
	metrics := &metricBatch{}
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	metrics.Metrics = generateMetrics(start)
	metrics.AttributesJSON = json.RawMessage(`{"zip":"zap"}`)

	expect := compactJSONString(`[{
		"common":{
			"attributes":{"zip":"zap"}
		},
		"metrics":[
			{
				"name":"mySummary",
				"type":"summary",
				"value":{"sum":15,"count":3,"min":4,"max":6},
				"timestamp":1417136460000,
				"interval.ms":5000,
				"attributes":{"attribute":"string"}
			},
			{
				"name":"myGauge",
				"type":"gauge",
				"value":12.3,
				"timestamp":1417136460000,
				"attributes":{"attribute":true}
			},
			{
				"name":"myCount",
				"type":"count",
				"value":100,
				"timestamp":1417136460000,
				"interval.ms":5000,
				"attributes":{"attribute":123}
			}
		]
	}]`)

	expectedContext := context.Background()
	reqs, err := newRequests(expectedContext, metrics, "my-api-key", defaultMetricURL, "userAgent")
	if err != nil {
		t.Error("error creating request", err)
	}
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	req := reqs[0]
	data := reqs[0].UncompressedBody
	if string(data) != expect {
		t.Error("metrics JSON mismatch", string(data), expect)
	}
	body, err := ioutil.ReadAll(req.Request.Body)
	req.Request.Body.Close()
	if err != nil {
		t.Fatal("unable to read body", err)
	}
	if len(body) != req.compressedBodyLength {
		t.Error("compressed body length mismatch", len(body), req.compressedBodyLength)
	}
	uncompressed, err := internal.Uncompress(body)
	if err != nil {
		t.Fatal("unable to uncompress body", err)
	}
	if string(uncompressed) != expect {
		t.Error("metrics JSON mismatch", string(uncompressed), expect)
	}
}

func generateMetrics(start time.Time) []Metric {
	return []Metric{
		Summary{
			Name: "mySummary",
			Attributes: map[string]interface{}{
				"attribute": "string",
			},
			Count:     3,
			Sum:       15,
			Min:       4,
			Max:       6,
			Timestamp: start,
			Interval:  5 * time.Second,
		},
		Gauge{
			Name: "myGauge",
			Attributes: map[string]interface{}{
				"attribute": true,
			},
			Value:     12.3,
			Timestamp: start,
		},
		Count{
			Name: "myCount",
			Attributes: map[string]interface{}{
				"attribute": 123,
			},
			Value:     100,
			Timestamp: start,
			Interval:  5 * time.Second,
		},
	}
}

func TestMetricBatch(t *testing.T) {
	var metricBatches []metricBatch
	for i := 0; i < 2; i++ {
		metrics := metricBatch{}
		start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
		metrics.Metrics = generateMetrics(start)
		metrics.Identity = fmt.Sprintf("Identity-%v", i)
		metrics.AttributesJSON = json.RawMessage(fmt.Sprintf(`{"zip":"zap_%v"}`, i))
		metricBatches = append(metricBatches, metrics)
	}

	requests, err := newBatchRequest(context.Background(), config{
		metricBatches,
		"my-api-key",
		defaultMetricURL,
		"userAgent",
		2,
	})
	require.NoError(t, err)
	require.Len(t, requests, 1)
	assert.Equal(t, "Identity-0,Identity-1", requests[0].Request.Header.Get("X-NRI-Entity-Ids"))

	expect := compactJSONString(`[
  {
    "common": {
      "attributes": {
        "zip": "zap_0"
      }
    },
    "metrics": [
      {
        "name": "mySummary",
        "type": "summary",
        "value": {
          "sum": 15,
          "count": 3,
          "min": 4,
          "max": 6
        },
        "timestamp": 1417136460000,
        "interval.ms": 5000,
        "attributes": {
          "attribute": "string"
        }
      },
      {
        "name": "myGauge",
        "type": "gauge",
        "value": 12.3,
        "timestamp": 1417136460000,
        "attributes": {
          "attribute": true
        }
      },
      {
        "name": "myCount",
        "type": "count",
        "value": 100,
        "timestamp": 1417136460000,
        "interval.ms": 5000,
        "attributes": {
          "attribute": 123
        }
      }
    ]
  },
  {
    "common": {
      "attributes": {
        "zip": "zap_1"
      }
    },
    "metrics": [
      {
        "name": "mySummary",
        "type": "summary",
        "value": {
          "sum": 15,
          "count": 3,
          "min": 4,
          "max": 6
        },
        "timestamp": 1417136460000,
        "interval.ms": 5000,
        "attributes": {
          "attribute": "string"
        }
      },
      {
        "name": "myGauge",
        "type": "gauge",
        "value": 12.3,
        "timestamp": 1417136460000,
        "attributes": {
          "attribute": true
        }
      },
      {
        "name": "myCount",
        "type": "count",
        "value": 100,
        "timestamp": 1417136460000,
        "interval.ms": 5000,
        "attributes": {
          "attribute": 123
        }
      }
    ]
  }
]
`)

	req := requests[0]
	data := req.UncompressedBody
	assert.Equal(t, expect, string(data))

	body, err := ioutil.ReadAll(req.Request.Body)
	require.NoError(t, req.Request.Body.Close())
	assert.Len(t, body, req.compressedBodyLength)
	uncompressed, err := internal.Uncompress(body)
	require.NoError(t, err)
	assert.Equal(t, expect, string(uncompressed))
}

func testBatchJSON(t testing.TB, batch *metricBatch, expect string) {
	if th, ok := t.(interface{ Helper() }); ok {
		th.Helper()
	}
	reqs, err := newRequests(context.Background(), batch, "my-api-key", defaultMetricURL, "userAgent")
	if nil != err {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	js := reqs[0].UncompressedBody
	actual := string(js)
	compactExpect := compactJSONString(expect)
	if actual != compactExpect {
		t.Errorf("\nexpect=%s\nactual=%s\n", compactExpect, actual)
	}
}

func TestSplit(t *testing.T) {
	// test len 0
	batch := &metricBatch{}
	split := batch.split()
	if split != nil {
		t.Error(split)
	}

	// test len 1
	batch = &metricBatch{Metrics: []Metric{Count{}}}
	split = batch.split()
	if split != nil {
		t.Error(split)
	}

	// test len 2
	batch = &metricBatch{Metrics: []Metric{Count{Name: "c1"}, Count{Name: "c2"}}}
	split = batch.split()
	if len(split) != 2 {
		t.Error("split into incorrect number of slices", len(split))
	}
	testBatchJSON(t, split[0].(*metricBatch), `[{"common":{},"metrics":[{"name":"c1","type":"count","value":0}]}]`)
	testBatchJSON(t, split[1].(*metricBatch), `[{"common":{},"metrics":[{"name":"c2","type":"count","value":0}]}]`)

	// test len 3
	batch = &metricBatch{Metrics: []Metric{Count{Name: "c1"}, Count{Name: "c2"}, Count{Name: "c3"}}}
	split = batch.split()
	if len(split) != 2 {
		t.Error("split into incorrect number of slices", len(split))
	}
	testBatchJSON(t, split[0].(*metricBatch), `[{"common":{},"metrics":[{"name":"c1","type":"count","value":0}]}]`)
	testBatchJSON(t, split[1].(*metricBatch), `[{"common":{},"metrics":[{"name":"c2","type":"count","value":0},{"name":"c3","type":"count","value":0}]}]`)
}

func BenchmarkMetricsJSON(b *testing.B) {
	// This benchmark tests the overhead of turning metrics into JSON.
	batch := &metricBatch{
		AttributesJSON: json.RawMessage(`{"zip": "zap"}`),
	}
	numMetrics := 10 * 1000
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)

	for i := 0; i < numMetrics/3; i++ {
		batch.Metrics = append(batch.Metrics, Summary{
			Name:       "mySummary",
			Attributes: map[string]interface{}{"attribute": "string"},
			Count:      3,
			Sum:        15,
			Min:        4,
			Max:        6,
			Timestamp:  start,
			Interval:   5 * time.Second,
		})
		batch.Metrics = append(batch.Metrics, Gauge{
			Name:       "myGauge",
			Attributes: map[string]interface{}{"attribute": true},
			Value:      12.3,
			Timestamp:  start,
		})
		batch.Metrics = append(batch.Metrics, Count{
			Name:       "myCount",
			Attributes: map[string]interface{}{"attribute": 123},
			Value:      100,
			Timestamp:  start,
			Interval:   5 * time.Second,
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	estimate := len(batch.Metrics) * 256
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(make([]byte, 0, estimate))
		batch.writeJSON(buf)
		bts := buf.Bytes()
		if len(bts) == 0 {
			b.Fatal(string(bts))
		}
	}
}

func TestMetricAttributesJSON(t *testing.T) {
	tests := []struct {
		key    string
		val    interface{}
		expect string
	}{
		{"string", "string", `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"string":"string"}}]}]`},
		{"true", true, `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"true":true}}]}]`},
		{"false", false, `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"false":false}}]}]`},
		{"uint8", uint8(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint8":1}}]}]`},
		{"uint16", uint16(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint16":1}}]}]`},
		{"uint32", uint32(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint32":1}}]}]`},
		{"uint64", uint64(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint64":1}}]}]`},
		{"uint", uint(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint":1}}]}]`},
		{"uintptr", uintptr(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"uintptr":1}}]}]`},
		{"int8", int8(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"int8":1}}]}]`},
		{"int16", int16(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"int16":1}}]}]`},
		{"int32", int32(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"int32":1}}]}]`},
		{"int64", int64(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"int64":1}}]}]`},
		{"int", int(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"int":1}}]}]`},
		{"float32", float32(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"float32":1}}]}]`},
		{"float64", float64(1), `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"float64":1}}]}]`},
		{"default", func() {}, `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"default":"func()"}}]}]`},
	}

	for _, test := range tests {
		batch := &metricBatch{}
		batch.Metrics = append(batch.Metrics, Count{
			Attributes: map[string]interface{}{
				test.key: test.val,
			},
		})
		testBatchJSON(t, batch, test.expect)
	}
}

func TestCountAttributesJSON(t *testing.T) {
	batch := &metricBatch{}
	batch.Metrics = append(batch.Metrics, Count{
		Attributes: map[string]interface{}{
			"zip": "zap",
		},
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, batch, `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"zip":"zap"}}]}]`)

	batch = &metricBatch{}
	batch.Metrics = append(batch.Metrics, Count{
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, batch, `[{"common":{},"metrics":[{"name":"","type":"count","value":0,"attributes":{"zing":"zang"}}]}]`)
}

func TestGaugeAttributesJSON(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)

	batch := &metricBatch{}
	batch.Metrics = append(batch.Metrics, Gauge{
		Attributes: map[string]interface{}{
			"zip": "zap",
		},
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
		Timestamp:      start,
	})
	testBatchJSON(t, batch, `[{"common":{},"metrics":[{"name":"","type":"gauge","value":0,"timestamp":1417136460000,"attributes":{"zip":"zap"}}]}]`)

	batch = &metricBatch{}
	batch.Metrics = append(batch.Metrics, Gauge{
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
		Timestamp:      start,
	})
	testBatchJSON(t, batch, `[{"common":{},"metrics":[{"name":"","type":"gauge","value":0,"timestamp":1417136460000,"attributes":{"zing":"zang"}}]}]`)
}

func TestSummaryAttributesJSON(t *testing.T) {
	batch := &metricBatch{}
	batch.Metrics = append(batch.Metrics, Summary{
		Attributes: map[string]interface{}{
			"zip": "zap",
		},
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, batch, `[{"common":{},"metrics":[{"name":"","type":"summary","value":{"sum":0,"count":0,"min":0,"max":0},"attributes":{"zip":"zap"}}]}]`)

	batch = &metricBatch{}
	batch.Metrics = append(batch.Metrics, Summary{
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, batch, `[{"common":{},"metrics":[{"name":"","type":"summary","value":{"sum":0,"count":0,"min":0,"max":0},"attributes":{"zing":"zang"}}]}]`)
}

func TestBatchAttributesJSON(t *testing.T) {
	batch := &metricBatch{
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	}
	testBatchJSON(t, batch, `[{"common":{"attributes":{"zing":"zang"}},"metrics":[]}]`)
}

func TestBatchStartEndTimesJSON(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)

	batch := &metricBatch{}
	testBatchJSON(t, batch, `[{"common":{},"metrics":[]}]`)

	batch = &metricBatch{
		Timestamp: start,
	}
	testBatchJSON(t, batch, `[{"common":{"timestamp":1417136460000},"metrics":[]}]`)

	batch = &metricBatch{
		Interval: 5 * time.Second,
	}
	testBatchJSON(t, batch, `[{"common":{"interval.ms":5000},"metrics":[]}]`)

	batch = &metricBatch{
		Timestamp: start,
		Interval:  5 * time.Second,
	}
	testBatchJSON(t, batch, `[{"common":{"timestamp":1417136460000,"interval.ms":5000},"metrics":[]}]`)
}

func TestCommonAttributes(t *testing.T) {
	// Tests when the "common" key is included in the metrics payload
	type testStruct struct {
		start          time.Time
		interval       time.Duration
		attributesJSON json.RawMessage
		expect         string
	}
	sometime := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	testcases := []testStruct{
		{expect: `[{"common":{},"metrics":[]}]`},
		{start: sometime, expect: `[{"common":{"timestamp":1417136460000},"metrics":[]}]`},
		{interval: 5 * time.Second, expect: `[{"common":{"interval.ms":5000},"metrics":[]}]`},
		{
			start: sometime, interval: 5 * time.Second,
			expect: `[{"common":{"timestamp":1417136460000,"interval.ms":5000},"metrics":[]}]`,
		},
		{
			attributesJSON: json.RawMessage(`{"zip":"zap"}`),
			expect:         `[{"common":{"attributes":{"zip":"zap"}},"metrics":[]}]`,
		},
	}

	for _, test := range testcases {
		batch := &metricBatch{
			Timestamp:      test.start,
			Interval:       test.interval,
			AttributesJSON: test.attributesJSON,
		}
		testBatchJSON(t, batch, test.expect)
	}
}

//nolint:funlen,exhaustruct
func Test_splitBatch(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		mb       metricBatch
		expected []metricBatch
	}{
		{
			name: "empty metric batch",
			mb: metricBatch{
				Identity: "some identity",
				Metrics:  nil,
			},
			expected: []metricBatch{
				{
					Identity: "some identity",
					Metrics:  nil,
				},
			},
		},
		{
			name: "single metric batch",
			mb: metricBatch{
				Identity: "some identity",
				Metrics: []Metric{
					Count{Name: "whatever", Value: 5.0},
				},
			},
			expected: []metricBatch{
				{
					Identity: "some identity",
					Metrics: []Metric{
						Count{Name: "whatever", Value: 5.0},
					},
				},
			},
		},
		{
			name: "even metric batch",
			mb: metricBatch{
				Identity: "some identity",
				Metrics: []Metric{
					Count{Name: "whatever1", Value: 1.0},
					Count{Name: "whatever2", Value: 2.0},
					Count{Name: "whatever3", Value: 3.0},
					Count{Name: "whatever4", Value: 4.0},
				},
			},
			expected: []metricBatch{
				{
					Identity: "some identity",
					Metrics: []Metric{
						Count{Name: "whatever1", Value: 1.0},
						Count{Name: "whatever2", Value: 2.0},
					},
				},
				{
					Identity: "some identity",
					Metrics: []Metric{
						Count{Name: "whatever3", Value: 3.0},
						Count{Name: "whatever4", Value: 4.0},
					},
				},
			},
		},
		{
			name: "odd metric batch",
			mb: metricBatch{
				Identity: "some identity",
				Metrics: []Metric{
					Count{Name: "whatever1", Value: 1.0},
					Count{Name: "whatever2", Value: 2.0},
					Count{Name: "whatever3", Value: 3.0},
					Count{Name: "whatever4", Value: 4.0},
					Count{Name: "whatever5", Value: 5.0},
				},
			},
			expected: []metricBatch{
				{
					Identity: "some identity",
					Metrics: []Metric{
						Count{Name: "whatever1", Value: 1.0},
						Count{Name: "whatever2", Value: 2.0},
					},
				},
				{
					Identity: "some identity",
					Metrics: []Metric{
						Count{Name: "whatever3", Value: 3.0},
						Count{Name: "whatever4", Value: 4.0},
						Count{Name: "whatever5", Value: 5.0},
					},
				},
			},
		},
	}
	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			splitted := testCase.mb.splitBatch()
			assert.Equal(t, testCase.expected, splitted)
		})
	}
}
