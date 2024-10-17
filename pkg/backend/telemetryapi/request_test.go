// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//nolint:exhaustruct
package telemetryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi/internal"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	logHelper "github.com/newrelic/infrastructure-agent/test/log"
	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRequestBuilder struct {
	bodies []json.RawMessage
}

func (ts testRequestBuilder) makeBody() json.RawMessage {
	return ts.bodies[0]
}

func (ts testRequestBuilder) split() []requestsBuilder {
	reqs := ts.bodies[1:]
	if len(reqs) == 0 {
		return nil
	}
	return []requestsBuilder{
		testRequestBuilder{bodies: reqs},
		testRequestBuilder{bodies: reqs},
	}
}

func TestNewRequestsSplitSuccess(t *testing.T) {
	ts := testRequestBuilder{
		bodies: []json.RawMessage{
			json.RawMessage(`12345678901234567890`),
			json.RawMessage(`123456789012345`),
			json.RawMessage(`12345678901`),
			json.RawMessage(`123456789`),
		},
	}
	reqs, err := newRequestsInternal(context.Background(), ts, "", "", "", func(r request) bool {
		return len(r.UncompressedBody) >= 10
	})
	if err != nil {
		t.Error(err)
	}
	if len(reqs) != 8 {
		t.Error(len(reqs))
	}
}

func TestNewRequestsCantSplit(t *testing.T) {
	ts := testRequestBuilder{
		bodies: []json.RawMessage{
			json.RawMessage(`12345678901234567890`),
			json.RawMessage(`123456789012345`),
			json.RawMessage(`12345678901`),
		},
	}
	reqs, err := newRequestsInternal(context.Background(), ts, "", "", "", func(r request) bool {
		return len(r.UncompressedBody) >= 10
	})
	if err != errUnableToSplit {
		t.Error(err)
	}
	if len(reqs) != 0 {
		t.Error(len(reqs))
	}
}

func randomJSON(numBytes int) json.RawMessage {
	digits := []byte{'1', '2', '3', '4', '5', '6', '7', '8', '9'}
	js := make([]byte, numBytes)
	for i := 0; i < len(js); i++ {
		js[i] = digits[rand.Intn(len(digits))]
	}
	return js
}

func TestLargeRequestNeedsSplit(t *testing.T) {
	js := randomJSON(4 * maxCompressedSizeBytes)
	reqs, err := newRequests(context.Background(), testRequestBuilder{bodies: []json.RawMessage{js}}, "apiKey", defaultMetricURL, "userAgent")
	if reqs != nil {
		t.Error(reqs)
	}
	if err != errUnableToSplit {
		t.Error(err)
	}
}

func TestLargeRequestNoSplit(t *testing.T) {
	js := randomJSON(maxCompressedSizeBytes / 2)
	reqs, err := newRequests(context.Background(), testRequestBuilder{bodies: []json.RawMessage{js}}, "apiKey", defaultMetricURL, "userAgent")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatal(len(reqs))
	}
	req := reqs[0]
	if u := req.Request.URL.String(); u != defaultMetricURL {
		t.Error(u)
	}
	if len(req.UncompressedBody) != maxCompressedSizeBytes/2 {
		t.Error(len(req.UncompressedBody))
	}
}

func Test_newBatchRequest(t *testing.T) {
	now := time.Now()
	type testRequest struct {
		xNRIEntityIdsHeader string
	}
	type args struct {
		metrics []metricBatch
	}
	tests := []struct {
		name     string
		args     args
		wantReqs []testRequest
		wantErr  bool
	}{
		{
			name: "basic",
			args: args{
				metrics: []metricBatch{
					{
						Identity:       "my-identity",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
				},
			},
			wantReqs: []testRequest{
				{xNRIEntityIdsHeader: "my-identity"},
			},
			wantErr: false,
		},
		{
			name: "multiple_batches",
			args: args{
				metrics: []metricBatch{
					{
						Identity:       "my-identity-one",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-two",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
				},
			},
			wantReqs: []testRequest{
				{xNRIEntityIdsHeader: "my-identity-one,my-identity-two"},
			},
			wantErr: false,
		},
		{
			name: "split-req-by-nr-of-entities",
			args: args{
				metrics: []metricBatch{
					{
						Identity:       "my-identity-one",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-two",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-three",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Gauge{
								Name:      "my_gauge",
								Value:     10,
								Timestamp: now,
							},
						},
					},
				},
			},
			wantReqs: []testRequest{
				{xNRIEntityIdsHeader: "my-identity-one,my-identity-two"},
				{xNRIEntityIdsHeader: "my-identity-three"},
			},
			wantErr: false,
		},
		{
			name: "split-high-nr-of-entities",
			args: args{
				metrics: []metricBatch{
					{
						Identity:       "my-identity-one",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-two",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-three",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Gauge{
								Name:      "my_gauge",
								Value:     10,
								Timestamp: now,
							},
						},
					},
					{
						Identity:       "my-identity-four",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-five",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-six",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Gauge{
								Name:      "my_gauge",
								Value:     10,
								Timestamp: now,
							},
						},
					},
					{
						Identity:       "my-identity-seven",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-eighth",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-nine",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Gauge{
								Name:      "my_gauge",
								Value:     10,
								Timestamp: now,
							},
						},
					},
					{
						Identity:       "my-identity-ten",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-eleven",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-twelve",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Gauge{
								Name:      "my_gauge",
								Value:     10,
								Timestamp: now,
							},
						},
					},
					{
						Identity:       "my-identity-thirteen",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-fourteen",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-fifteen",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Gauge{
								Name:      "my_gauge",
								Value:     10,
								Timestamp: now,
							},
						},
					},
					{
						Identity:       "my-identity-sixteen",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-seventeen",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-eighteen",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Gauge{
								Name:      "my_gauge",
								Value:     10,
								Timestamp: now,
							},
						},
					},
				},
			},
			wantReqs: []testRequest{
				{xNRIEntityIdsHeader: "my-identity-one,my-identity-two"},
				{xNRIEntityIdsHeader: "my-identity-three,my-identity-four"},
				{xNRIEntityIdsHeader: "my-identity-five,my-identity-six"},
				{xNRIEntityIdsHeader: "my-identity-seven,my-identity-eighth"},
				{xNRIEntityIdsHeader: "my-identity-nine,my-identity-ten"},
				{xNRIEntityIdsHeader: "my-identity-eleven,my-identity-twelve"},
				{xNRIEntityIdsHeader: "my-identity-thirteen,my-identity-fourteen"},
				{xNRIEntityIdsHeader: "my-identity-fifteen,my-identity-sixteen"},
				{xNRIEntityIdsHeader: "my-identity-seventeen,my-identity-eighteen"},
			},
			wantErr: false,
		},
		{
			name: "set-header-by-unique-entity-id",
			args: args{
				metrics: []metricBatch{
					{
						Identity:       "my-identity-one",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Count{
								Name:      "my_count",
								Value:     10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
					{
						Identity:       "my-identity-one",
						Timestamp:      now,
						Interval:       101,
						AttributesJSON: json.RawMessage(`12345678901234567890`),
						Metrics: []Metric{
							Summary{
								Name:      "my_summary",
								Count:     1,
								Sum:       10,
								Timestamp: now,
								Interval:  101,
							},
						},
					},
				},
			},
			wantReqs: []testRequest{
				{xNRIEntityIdsHeader: "my-identity-one"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedAPIKey := "apiKey_" + tt.name
			expectedURL := "http://url/" + tt.name
			expectedUserAgent := "userAgent/" + tt.name
			expectedContext := context.Background()
			gotReqs, err := newBatchRequest(expectedContext, config{
				data:        tt.args.metrics,
				apiKey:      expectedAPIKey,
				url:         expectedURL,
				userAgent:   expectedUserAgent,
				maxEntities: 2,
			})
			if !tt.wantErr {
				require.NoError(t, err)
			}
			assert.Len(t, gotReqs, len(tt.wantReqs))
			for i := range tt.wantReqs {
				assert.Equal(t, expectedAPIKey, gotReqs[i].Request.Header.Get("Api-Key"))
				assert.Equal(t, expectedURL, gotReqs[i].Request.URL.String())
				assert.Equal(t, expectedUserAgent, gotReqs[i].Request.Header.Get("User-Agent"))
				assert.Equal(t, tt.wantReqs[i].xNRIEntityIdsHeader, gotReqs[i].Request.Header.Get("X-NRI-Entity-Ids"))
				assert.Equal(t, expectedContext, gotReqs[i].Request.Context())
			}
		})
	}
}

func buildMetricForTest(attributes map[string]interface{}) *Count {
	return &Count{
		Name:       "SomeMetric",
		Attributes: attributes,
		Value:      10,
	}
}

func buildMetricsForTest(mbsAmount int, metricAmount int, attributes map[string]interface{}) []metricBatch {
	var mbs []metricBatch

	for i := 0; i < mbsAmount; i++ { //nolint:varnamelen
		var metrics []Metric
		for j := 0; j < metricAmount; j++ {
			metrics = append(metrics, buildMetricForTest(attributes))
		}
		mb := metricBatch{
			Identity: fmt.Sprintf("some identity %d", i),
			Metrics:  metrics,
		}
		mbs = append(mbs, mb)
	}

	return mbs
}

type metricPayload struct {
	Type  string      `json:"type"`
	Value float32     `json:"value"`
	Attrs interface{} `json:"attributes"`
}
type metricsPayload struct {
	Common  map[string]interface{} `json:"common"`
	Metrics []metricPayload        `json:"metrics"`
}

//nolint:funlen,paralleltest
func Test_buildRequestsMultipleMetricsBatch(t *testing.T) {
	attributes := map[string]interface{}{"some": "attribute"}

	origMaxSize := maxCompressedSizeBytes
	defer func() {
		maxCompressedSizeBytes = origMaxSize
	}()

	testCases := []struct {
		name             string
		mbsAmount        int
		metricAmount     int
		attrsSize        int
		maxLimitSize     func(batchSize int) int
		expectedRequests [][]int
	}{
		{
			name:         "no metric batch should not be splitted",
			mbsAmount:    0,
			metricAmount: 0,
			maxLimitSize: func(msize int) int {
				return origMaxSize
			},
			expectedRequests: [][]int{{}},
		},
		{
			name:         "one small metric batch should not be splitted",
			mbsAmount:    1,
			metricAmount: 12,
			maxLimitSize: func(msize int) int {
				return origMaxSize
			},
			expectedRequests: [][]int{{12}},
		},
		{
			name:         "multiple batches with small metrics should not be splitted",
			mbsAmount:    5,
			metricAmount: 12,
			maxLimitSize: func(msize int) int {
				return origMaxSize
			},
			expectedRequests: [][]int{{12, 12, 12, 12, 12}},
		},
		{
			name:         "one batch with big odd metrics bigger than limit should be splitted",
			mbsAmount:    1,
			metricAmount: 12,
			maxLimitSize: func(msize int) int {
				return msize - 10
			},
			expectedRequests: [][]int{{6}, {6}},
		},
		{
			name:         "one batch with big even metrics bigger than limit should be splitted",
			mbsAmount:    1,
			metricAmount: 13,
			maxLimitSize: func(msize int) int {
				return msize - 10
			},
			expectedRequests: [][]int{{6}, {7}},
		},
		{
			name:         "one batch with more odd big metrics bigger than limit should be splitted",
			mbsAmount:    1,
			metricAmount: 20,
			maxLimitSize: func(msize int) int {
				return ((msize / 20) * 4) + 120 // splitted in 10 + buffer
			},
			expectedRequests: [][]int{{5}, {5}, {5}, {5}},
		},
		{
			name:         "two batches with more even big metrics bigger than limit should be splitted",
			mbsAmount:    2,
			metricAmount: 20,
			maxLimitSize: func(msize int) int {
				return ((msize / 40) * 10) + 40 // splitted in 10 + buffer
			},
			expectedRequests: [][]int{{5}, {5}, {5}, {5}, {5}, {5}, {5}, {5}},
		},
		{
			name:         "two batches with big metrics bigger than limit should be splitted",
			mbsAmount:    2,
			metricAmount: 21,
			maxLimitSize: func(msize int) int {
				return ((msize / 42) * 10) + 40 // splitted in 10 + buffer
			},
			expectedRequests: [][]int{{5}, {5}, {5}, {3}, {3}, {5}, {5}, {5}, {3}, {3}},
		},
	}

	// non important stuff
	ctx := context.Background()
	apiKey := "some api key"
	url := "some url"
	userAgent := "some user agent"

	defer func() {
		compressFunc = internal.Compress
	}()
	compressFunc = func(b []byte) (*bytes.Buffer, error) {
		buf := bytes.Buffer{}

		_, err := buf.Write(b)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}

		return &buf, nil
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			// build metrics to get the payload size and set the max size
			mbs := buildMetricsForTest(testCase.mbsAmount, testCase.metricAmount, attributes)
			if len(mbs) > 0 {
				buf := new(bytes.Buffer)
				mbs[0].writeJSON(buf)
				msize := buf.Len()
				maxCompressedSizeBytes = testCase.maxLimitSize(msize)
			}

			// build requests
			reqs, err := buildRequests(ctx, mbs, apiKey, url, userAgent)
			assert.NoError(t, err)
			// assert the requests and metrics inside each request
			assert.Len(t, reqs, len(testCase.expectedRequests))
			for j := 0; j < len(testCase.expectedRequests); j++ { //nolint:varnamelen
				var reqPayload []metricsPayload
				err = json.Unmarshal(reqs[j].compressedBody, &reqPayload)
				assert.NoError(t, err)
				assert.Equal(t, len(testCase.expectedRequests[j]), len(reqPayload))
				for k := 0; k < len(testCase.expectedRequests[j]); k++ {
					assert.Len(t, reqPayload[k].Metrics, testCase.expectedRequests[j][k])
				}
			}
		})
	}
}
func TestCreateRequest(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		rawJSON                json.RawMessage
		compressedPayload      *bytes.Buffer
		compressedLen          int
		apiKey, url, userAgent string
	}{
		"successful request creation": {
			rawJSON:           json.RawMessage(`{"key":"value"}`),
			compressedPayload: bytes.NewBufferString("compressed data"),
			compressedLen:     14,
			apiKey:            "test-api-key",
			url:               "http://test-url",
			userAgent:         "test-user-agent",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			// Mock compressFunc
			origCompressFunc := compressFunc
			defer func() { compressFunc = origCompressFunc }()
			compressFunc = func(_ []byte) (*bytes.Buffer, error) {
				return test.compressedPayload, nil
			}
			// Mock log
			hook := logHelper.NewInMemoryEntriesHook([]logrus.Level{logrus.DebugLevel})
			log.AddHook(hook)
			log.SetLevel(logrus.TraceLevel)
			// Test
			req, err := createRequest(ctx, test.rawJSON, test.compressedPayload, test.compressedLen, test.apiKey, test.url, test.userAgent)
			// Assert
			require.NoError(t, err)
			assert.Equal(t, "application/json", req.Request.Header.Get("Content-Type"))
			assert.Equal(t, test.apiKey, req.Request.Header.Get("Api-Key"))
			assert.Equal(t, "gzip", req.Request.Header.Get("Content-Encoding"))
			assert.Equal(t, test.userAgent, req.Request.Header.Get("User-Agent"))
			assert.Equal(t, test.url, req.Request.URL.String())
			assert.Equal(t, ctx, req.Request.Context())
			assert.Equal(t, test.rawJSON, req.UncompressedBody)
			assert.Equal(t, test.compressedPayload.Bytes(), req.compressedBody)
			assert.Equal(t, test.compressedLen, req.compressedBodyLength)
			// Assert log
			assert.True(t, hook.EntryWithMessageExists(regexp.MustCompile(`Request created`)))
			logEntries := hook.GetEntries()
			assert.Len(t, logEntries, 1)
			assert.Equal(t, logrus.DebugLevel, logEntries[0].Level)
			assert.Equal(t, test.compressedLen, logEntries[0].Data["compressed_data_size"])
		})
	}
}
