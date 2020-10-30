// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
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
					}, {
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
					}, {
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
					}, {
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
					}, {
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
					}, {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxEntitiesByRequest = 2
			expectedAPIKey := "apiKey_" + tt.name
			expectedURL := "http://url/" + tt.name
			expectedUserAgent := "userAgent/" + tt.name
			expectedContext := context.Background()
			gotReqs, err := newBatchRequest(expectedContext, tt.args.metrics, expectedAPIKey, expectedURL, expectedUserAgent)
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
