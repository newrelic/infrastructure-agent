// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi/internal"
)

// compactJSONString removes the whitespace from a JSON string.  This function
// will panic if the string provided is not valid JSON.
func compactJSONString(js string) string {
	buf := new(bytes.Buffer)
	if err := json.Compact(buf, []byte(js)); err != nil {
		panic(fmt.Errorf("unable to compact JSON: %v", err))
	}
	return buf.String()
}

func TestNilHarvestNow(t *testing.T) {
	var h *Harvester
	h.HarvestNow(context.Background())
}

func TestNilHarvesterRecordSpan(t *testing.T) {
	var h *Harvester
	h.RecordSpan(Span{
		ID:          "id",
		TraceID:     "traceId",
		Name:        "myspan",
		ParentID:    "parentId",
		Timestamp:   time.Now(),
		Duration:    time.Second,
		ServiceName: "span",
		Attributes: map[string]interface{}{
			"attr": 1,
		},
	})
}

func TestHarvestErrorLogger(t *testing.T) {
	err := map[string]interface{}{}

	harvestMissingErrorLogger, _ := NewHarvester(configTesting)
	harvestMissingErrorLogger.config.logError(err)

	var savedErrors []map[string]interface{}
	h, _ := NewHarvester(configTesting, func(cfg *Config) {
		cfg.ErrorLogger = func(e map[string]interface{}) {
			savedErrors = append(savedErrors, e)
		}
	})
	h.config.logError(err)
	if len(savedErrors) != 1 {
		t.Error("incorrect errors found", savedErrors)
	}
}

func TestHarvestDebugLogger(t *testing.T) {
	fields := map[string]interface{}{
		"something": "happened",
	}

	emptyHarvest, _ := NewHarvester(configTesting)
	emptyHarvest.config.logDebug(fields)

	var savedFields map[string]interface{}
	h, _ := NewHarvester(configTesting, func(cfg *Config) {
		cfg.DebugLogger = func(f map[string]interface{}) {
			savedFields = f
		}
	})
	h.config.logDebug(fields)
	if !reflect.DeepEqual(fields, savedFields) {
		t.Error(fields, savedFields)
	}
}

func TestVetCommonAttributes(t *testing.T) {
	attributes := map[string]interface{}{
		"bool":           true,
		"bad":            struct{}{},
		"int":            123,
		"remove-me":      t,
		"nil-is-invalid": nil,
	}
	var savedErrors []map[string]interface{}
	NewHarvester(
		configTesting,
		ConfigCommonAttributes(attributes),
		func(cfg *Config) {
			cfg.ErrorLogger = func(e map[string]interface{}) {
				savedErrors = append(savedErrors, e)
			}
		},
	)
	if len(savedErrors) != 3 {
		t.Fatal(savedErrors)
	}
}

func TestHarvestCancelled(t *testing.T) {
	errs := int32(0)
	posts := int32(0)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		go func() {
			time.Sleep(time.Second)
			wg.Done()
		}()
		// Test that the context with the deadline is added to the
		// harvest request.
		<-r.Context().Done()
		atomic.AddInt32(&posts, 1)

		// Set a retry after so that the backoff sleep is not zero to
		// ensure that the Context().Done() select always succeeds.
		return &http.Response{
			Header:     http.Header{"Retry-After": []string{"2"}},
			StatusCode: 429,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	})
	h, _ := NewHarvester(func(cfg *Config) {
		cfg.ErrorLogger = func(e map[string]interface{}) {
			atomic.AddInt32(&errs, 1)
		}
		cfg.HarvestPeriod = 0
		cfg.Client.Transport = rt
		cfg.APIKey = "key"
		cfg.Context = context.Background()
	})
	h.RecordSpan(Span{TraceID: "id", ID: "id"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h.HarvestNow(ctx)
	wg.Wait()

	actualPosts := atomic.LoadInt32(&posts)
	if actualPosts != 1 {
		t.Error("incorrect number of tries tried", actualPosts)
	}
	actualErrors := atomic.LoadInt32(&errs)
	if actualErrors != 2 {
		t.Error("incorrect number of errors logged", actualErrors)
	}
}

func TestNewRequestHeaders(t *testing.T) {
	h, _ := NewHarvester(configTesting, func(cfg *Config) {
		cfg.Product = "myProduct"
		cfg.ProductVersion = "0.1.0"
	})
	expectUserAgent := "NewRelic-Go-TelemetrySDK/" + version + " myProduct/0.1.0"
	h.RecordSpan(Span{TraceID: "id", ID: "id"})
	h.RecordMetric(Gauge{})

	expectedContext := context.Background()
	reqs := h.swapOutSpans(expectedContext)
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	req := reqs[0]
	if h := req.Request.Header.Get("Content-Encoding"); "gzip" != h {
		t.Error("incorrect Content-Encoding header", req.Request.Header)
	}
	if h := req.Request.Header.Get("User-Agent"); expectUserAgent != h {
		t.Error("User-Agent header incorrect", req.Request.Header)
	}

	reqs = h.swapOutMetrics(h.config.Context, time.Now())
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	req = reqs[0]
	if h := req.Request.Header.Get("Content-Type"); "application/json" != h {
		t.Error("incorrect Content-Type", h)
	}
	if h := req.Request.Header.Get("Api-Key"); "api-key" != h {
		t.Error("incorrect Api-Key", h)
	}
	if h := req.Request.Header.Get("Content-Encoding"); "gzip" != h {
		t.Error("incorrect Content-Encoding header", h)
	}
	if h := req.Request.Header.Get("User-Agent"); expectUserAgent != h {
		t.Error("User-Agent header incorrect", h)
	}
	if req.Request.Context() != expectedContext {
		t.Error("incorrect context being used")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// optional interface required for go1.4 and go1.5
func (fn roundTripperFunc) CancelRequest(*http.Request) {}

func emptyResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(""))),
	}
}

func uncompressBody(req *http.Request) (string, error) {
	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		return "", fmt.Errorf("unable to read body: %v", err)
	}
	uncompressed, err := internal.Uncompress(body)
	if err != nil {
		return "", fmt.Errorf("unable to uncompress body: %v", err)
	}
	return string(uncompressed), nil
}

// sortedMetricsHelper is used to sort metrics for JSON comparison.
type sortedMetricsHelper []json.RawMessage

func (h sortedMetricsHelper) Len() int {
	return len(h)
}
func (h sortedMetricsHelper) Less(i, j int) bool {
	return string(h[i]) < string(h[j])
}
func (h sortedMetricsHelper) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func testHarvesterMetrics(t testing.TB, h *Harvester, expect string) {
	reqs := h.swapOutMetrics(context.Background(), time.Now())
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	if u := reqs[0].Request.URL.String(); u != defaultMetricURL {
		t.Error(u)
	}
	js := reqs[0].UncompressedBody
	var helper []struct {
		Metrics sortedMetricsHelper `json:"metrics"`
	}
	if err := json.Unmarshal(js, &helper); err != nil {
		t.Fatal("unable to unmarshal metrics for sorting", err)
		return
	}
	sort.Sort(helper[0].Metrics)
	js, err := json.Marshal(helper[0].Metrics)
	if nil != err {
		t.Fatal("unable to marshal metrics", err)
		return
	}
	actual := string(js)

	if th, ok := t.(interface{ Helper() }); ok {
		th.Helper()
	}
	compactExpect := compactJSONString(expect)
	if compactExpect != actual {
		t.Errorf("\nexpect=%s\nactual=%s\n", compactExpect, actual)
	}
}

func TestRecordMetric(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	h, _ := NewHarvester(configTesting)
	h.RecordMetric(Count{
		Name:           "myCount",
		AttributesJSON: json.RawMessage(`{"zip":"zap"}`),
		Value:          123,
		Timestamp:      start,
		Interval:       5 * time.Second,
	})
	h.RecordMetric(Gauge{
		Name:       "myGauge",
		Attributes: map[string]interface{}{"zippity": "zappity"},
		Value:      246,
		Timestamp:  start,
	})
	h.RecordMetric(Summary{
		Name:       "mySummary",
		Attributes: map[string]interface{}{"zup": "zop"},
		Count:      3,
		Sum:        15,
		Min:        4,
		Max:        6,
		Timestamp:  start,
		Interval:   5 * time.Second,
	})
	expect := `[
		{"name":"myCount","type":"count","value":123,"timestamp":1417136460000,"interval.ms":5000,"attributes":{"zip":"zap"}},
		{"name":"myGauge","type":"gauge","value":246,"timestamp":1417136460000,"attributes":{"zippity":"zappity"}},
		{"name":"mySummary","type":"summary","value":{"sum":15,"count":3,"min":4,"max":6},"timestamp":1417136460000,"interval.ms":5000,"attributes":{"zup":"zop"}}
	]`
	testHarvesterMetrics(t, h, expect)
}

func TestReturnCodes(t *testing.T) {
	// tests which return codes should retry and which should not
	testcases := []struct {
		returnCode  int
		shouldRetry bool
	}{
		{200, false},
		{202, false},
		{400, false},
		{403, false},
		{404, false},
		{405, false},
		{411, false},
		{413, false},
		{429, true},
		{500, true},
		{503, true},
	}

	for _, test := range testcases {
		t.Run(fmt.Sprintf("%v_%v", test.returnCode, test.shouldRetry), func(t *testing.T) {
			posts := 0
			wg := &sync.WaitGroup{}
			if test.shouldRetry {
				wg.Add(2)
			} else {
				wg.Add(1)
			}
			sp := Span{TraceID: "id", ID: "id", Name: "span1", Timestamp: time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)}

			rtFunc := func(code int) roundTripperFunc {
				return func(req *http.Request) (*http.Response, error) {
					posts++
					wg.Done()
					if posts > 1 {
						return emptyResponse(202), nil
					}
					return emptyResponse(code), nil
				}
			}
			h, _ := NewHarvester(configTesting, func(cfg *Config) {
				cfg.Client.Transport = rtFunc(test.returnCode)
			})

			h.RecordSpan(sp)
			h.HarvestNow(context.Background())

			if testTimesout(wg) {
				t.Error("incorrect number of posts", posts)
			}

			if (test.shouldRetry && 2 != posts) || (!test.shouldRetry && 1 != posts) {
				t.Error("incorrect number of posts", posts)
			}
		})
	}
}

func testTimesout(wg *sync.WaitGroup) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false
	case <-time.After(time.Second):
		return true
	}
}

func Test429RetryAfterUsesConfig(t *testing.T) {
	// Test when resp code is 429, retry backoff uses value from config if:
	// * Retry-After header not set
	// * Retry-After header not parsable
	// * Retry-After header delay is less than config retry backoff
	testcases := []struct {
		name        string
		retryHeader string
	}{
		{name: "Retry-After header not set", retryHeader: ""},
		{name: "Retry-After header not parsable", retryHeader: "hello world!"},
		{name: "Retry-After header delay is less than config retry backoff", retryHeader: "0"},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			var posts int
			wg := &sync.WaitGroup{}
			wg.Add(2)
			var start time.Time
			tm := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
			span := Span{TraceID: "id", ID: "id", Name: "span1", Timestamp: tm}

			roundTripper := func(retryHeader string) roundTripperFunc {
				return func(req *http.Request) (*http.Response, error) {
					posts++
					wg.Done()
					if posts > 1 {
						if since := time.Since(start); since > time.Second {
							t.Errorf("incorrect retry backoff used, since=%v", since)
						}
						return emptyResponse(200), nil
					}
					start = time.Now()
					resp := emptyResponse(429)
					resp.Header = http.Header{}
					resp.Header.Add("Retry-After", retryHeader)
					return resp, nil
				}
			}

			h, _ := NewHarvester(func(cfg *Config) {
				cfg.Client.Transport = roundTripper("")
				cfg.APIKey = "key"
			})
			h.RecordSpan(span)
			h.HarvestNow(context.Background())
			if testTimesout(wg) {
				t.Error("incorrect number of posts", posts)
			}
			if posts != 2 {
				t.Error("incorrect number of posts", posts)
			}
		})
	}
}

func TestResponseNeedsRetry(t *testing.T) {
	testcases := []struct {
		attempts      int
		headerRetry   string
		respCode      int
		expectRetry   bool
		expectBackoff time.Duration
	}{
		{
			attempts:      0,
			headerRetry:   "2",
			respCode:      202,
			expectRetry:   false,
			expectBackoff: 0,
		},
		{
			attempts:      0,
			headerRetry:   "2",
			respCode:      200,
			expectRetry:   false,
			expectBackoff: 0,
		},
		{
			attempts:      0,
			headerRetry:   "",
			respCode:      500,
			expectRetry:   true,
			expectBackoff: 0,
		},
		{
			attempts:      1,
			headerRetry:   "",
			respCode:      500,
			expectRetry:   true,
			expectBackoff: time.Second,
		},
		{
			attempts:      2,
			headerRetry:   "",
			respCode:      500,
			expectRetry:   true,
			expectBackoff: 2 * time.Second,
		},
		{
			attempts:      3,
			headerRetry:   "",
			respCode:      500,
			expectRetry:   true,
			expectBackoff: 4 * time.Second,
		},
		{
			attempts:      500,
			headerRetry:   "",
			respCode:      500,
			expectRetry:   true,
			expectBackoff: 16 * time.Second,
		},
		{
			attempts:      0,
			headerRetry:   "2",
			respCode:      413,
			expectRetry:   false,
			expectBackoff: 0,
		},
		{
			attempts:      1,
			headerRetry:   "",
			respCode:      429,
			expectRetry:   true,
			expectBackoff: time.Second,
		},
		{
			attempts:      1,
			headerRetry:   "hello",
			respCode:      429,
			expectRetry:   true,
			expectBackoff: time.Second,
		},
		{
			attempts:      1,
			headerRetry:   "0.5",
			respCode:      429,
			expectRetry:   true,
			expectBackoff: time.Second,
		},
		{
			attempts:      1,
			headerRetry:   "2",
			respCode:      429,
			expectRetry:   true,
			expectBackoff: 2 * time.Second,
		},
	}

	h, _ := NewHarvester(configTesting)
	for _, test := range testcases {
		resp := response{
			statusCode: test.respCode,
			retryAfter: test.headerRetry,
		}
		actualRetry, actualBackoff := resp.needsRetry(&h.config, test.attempts)
		if actualRetry != test.expectRetry {
			t.Errorf("incorrect retry value found, actualRetry=%t, expectRetry=%t", actualRetry, test.expectRetry)
		}
		if actualBackoff != test.expectBackoff {
			t.Errorf("incorrect retry value found, actualBackoff=%v, expectBackoff=%v", actualBackoff, test.expectBackoff)
		}
	}
}

func TestNoDataNoHarvest(t *testing.T) {
	roundTripper := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		t.Error("harvest should not have been run")
		return emptyResponse(200), nil
	})

	h, _ := NewHarvester(func(cfg *Config) {
		cfg.HarvestPeriod = 0
		cfg.Client.Transport = roundTripper
		cfg.APIKey = "APIKey"
	})
	h.HarvestNow(context.Background())
}

func TestNewRequestErrorNoPost(t *testing.T) {
	// Test that when newRequest returns an error, no post is made
	roundTripper := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		t.Error("no post should not have been run")
		return emptyResponse(200), nil
	})

	h, _ := NewHarvester(func(cfg *Config) {
		cfg.HarvestPeriod = 0
		cfg.Client.Transport = roundTripper
		cfg.APIKey = "APIKey"
		cfg.MetricsURLOverride = "t h i s  i s  n o t  a  h o s t%"
	})
	h.RecordMetric(Count{})
	h.HarvestNow(context.Background())
}

func TestRecordMetricNil(t *testing.T) {
	var h *Harvester
	h.RecordMetric(Count{})
}

func TestRecordSpanZeroTimestamp(t *testing.T) {
	h, _ := NewHarvester(func(cfg *Config) {
		cfg.HarvestPeriod = 0
		cfg.APIKey = "APIKey"
	})
	if err := h.RecordSpan(Span{
		ID:      "id",
		TraceID: "traceid",
	}); err != nil {
		t.Fatal(err)
	}
	if s := h.spans[0]; s.Timestamp.IsZero() {
		t.Fatal(s.Timestamp)
	}
}

func TestHarvestAuditLog(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	roundTripper := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		defer wg.Done()
		return emptyResponse(200), nil
	})

	var audit map[string]interface{}

	h, _ := NewHarvester(func(cfg *Config) {
		cfg.HarvestPeriod = 0
		cfg.APIKey = "APIKey"
		cfg.Client.Transport = roundTripper
		cfg.AuditLogger = func(fields map[string]interface{}) {
			audit = fields
		}
	})
	h.RecordMetric(Count{})
	h.HarvestNow(context.Background())

	if testTimesout(wg) {
		t.Error("Test timed out")
	}
	if u := audit["url"]; u != "https://metric-api.newrelic.com/metric/v1" {
		t.Fatal(u)
	}
	// We can't test "data" against a fixed string because of the dynamic
	// timestamp.
	if d := audit["data"]; !strings.Contains(string(d.(jsonString)), `"metrics":[{"name":"","type":"count","value":0}]`) {
		t.Fatal(d)
	}
}

func TestRequiredSpanFields(t *testing.T) {
	h, _ := NewHarvester(configTesting)
	if err := h.RecordSpan(Span{ID: "12345"}); err != errTraceIDUnset {
		t.Error(err)
	}
	if err := h.RecordSpan(Span{TraceID: "12345"}); err != errSpanIDUnset {
		t.Error(err)
	}
}

func TestRecordInvalidMetric(t *testing.T) {
	var savedErrors []map[string]interface{}
	h, _ := NewHarvester(configTesting, configSaveErrors(&savedErrors))
	h.RecordMetric(Count{
		Name:  "bad-metric",
		Value: math.NaN(),
	})
	if len(savedErrors) != 1 || !reflect.DeepEqual(savedErrors[0], map[string]interface{}{
		"err":     errFloatNaN.Error(),
		"message": "invalid count value",
		"name":    "bad-metric",
	}) {
		t.Error(savedErrors)
	}
	if len(h.rawMetrics) != 0 {
		t.Error(len(h.rawMetrics))
	}
}

func TestRequestRetryBody(t *testing.T) {
	var attempt int
	roundTripper := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		contentLen := int(req.ContentLength)
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Fatal("error reading request body: ", err)
		}
		bodyLen := len(body)
		if contentLen != bodyLen {
			t.Errorf("content-length and body length mis-match: content=%d body=%d",
				contentLen, bodyLen)
		}

		attempt++
		if attempt < 2 {
			return emptyResponse(418), nil
		}
		return emptyResponse(200), nil
	})

	h, _ := NewHarvester(func(cfg *Config) {
		cfg.HarvestPeriod = 0
		cfg.APIKey = "APIKey"
		cfg.Client.Transport = roundTripper
	})
	h.RecordMetric(Count{})
	h.HarvestNow(context.Background())
}

// multiAttemptRoundTripper will fail the first n requests after reading
// their body with a 418. Subsequent requests will be returned a 200.
func multiAttemptRoundTripper(n int) roundTripperFunc {
	var attempt int
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		defer func() { attempt++ }()
		if _, err := ioutil.ReadAll(req.Body); err != nil {
			return nil, err
		}
		if attempt < n {
			return emptyResponse(418), nil
		}
		return emptyResponse(200), nil
	})
}

func benchmarkRetryBodyN(b *testing.B, n int) {
	// Disable backoff delay.
	oBOSS := backoffSequenceSeconds
	backoffSequenceSeconds = make([]int, n+1)

	count := Count{}
	ctx := context.Background()
	h, _ := NewHarvester(func(cfg *Config) {
		cfg.HarvestPeriod = 0
		cfg.APIKey = "APIKey"
		cfg.Client.Transport = multiAttemptRoundTripper(n)
	})

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h.RecordMetric(count)
		h.HarvestNow(ctx)
	}

	b.StopTimer()
	backoffSequenceSeconds = oBOSS
}

// Baseline for the rest. This does not retry.
func BenchmarkRetryBody0(b *testing.B) { benchmarkRetryBodyN(b, 0) }
func BenchmarkRetryBody1(b *testing.B) { benchmarkRetryBodyN(b, 1) }
func BenchmarkRetryBody2(b *testing.B) { benchmarkRetryBodyN(b, 2) }
func BenchmarkRetryBody4(b *testing.B) { benchmarkRetryBodyN(b, 4) }
func BenchmarkRetryBody8(b *testing.B) { benchmarkRetryBodyN(b, 8) }
