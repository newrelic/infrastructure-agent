// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package inventoryapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"

	. "gopkg.in/check.v1"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
)

func Test(t *testing.T) { TestingT(t) }

type IngestAPISuite struct{}

var _ = Suite(&IngestAPISuite{})

func (*IngestAPISuite) TestCreatePostDeltaResponse(c *C) {
	pdr := NewPostDeltaResponse()

	c.Assert(pdr.StateMap, Not(IsNil))
}

func (*IngestAPISuite) TestCreateRawDeltaFull(c *C) {
	var jsonBlob = []byte(`{
		"key1" : "value1", "key2" : "value2"
	}`)
	var diff map[string]interface{}
	err := json.Unmarshal(jsonBlob, &diff)
	c.Assert(err, IsNil)
	rd := NewRawDelta("source", 123, 456, diff, true)

	c.Assert(rd.Source, Equals, "source")
	c.Assert(rd.ID, Equals, int64(123))
	c.Assert(rd.Timestamp, Equals, int64(456))
	c.Assert(rd.Diff, DeepEquals, diff)
	c.Assert(rd.FullDiff, Equals, true)
}

func (*IngestAPISuite) TestCreateRawDeltaPartial(c *C) {
	var jsonBlob = []byte(`{
		"key1" : "value1", "key2" : "value2"
	}`)
	var diff map[string]interface{}
	err := json.Unmarshal(jsonBlob, &diff)
	c.Assert(err, IsNil)
	rd := NewRawDelta("source", 123, 456, diff, false)

	c.Assert(rd.FullDiff, Equals, false)
}

func (*IngestAPISuite) TestMakeURLAccountPrefix(c *C) {
	client, _ := NewIngestClient("http://test.com", "abc", "useragent", 0, "", nil, false, backendhttp.NullHttpClient)
	url := client.makeURL("/mypath")
	c.Assert(url, Equals, "http://test.com/mypath")
}

func (*IngestAPISuite) TestMakeURLAccountPrefixTrimmed(c *C) {
	client, _ := NewIngestClient("http://test.com/inventory/", "abc", "useragent", 0, "", nil, false, backendhttp.NullHttpClient)
	url := client.makeURL("/mypath")
	c.Assert(url, Equals, "http://test.com/inventory/mypath")
}

func setupMockedClient(code int, body []byte) *IngestClient {
	mock := backendhttp.NewMockTransport()
	mock.Append(code, body)
	clientMock := http.Client{Transport: mock}

	return &IngestClient{
		svcUrl:     "http://test.com",
		licenseKey: "abc",
		HttpClient: clientMock.Do,
		userAgent:  "agentsmith",
	}
}

func (*IngestAPISuite) TestLicenseKeyIsAttached(c *C) {
	client := setupMockedClient(202, []byte(`{}`))

	req, err := http.NewRequest("POST", "http://test.com/deltas", nil)
	c.Assert(err, IsNil)
	client.Do(req)

	c.Assert(req.Header.Get(backendhttp.LicenseHeader), Equals, client.licenseKey)
}

func (*IngestAPISuite) TestEntityKeyIsAttached(c *C) {
	client := setupMockedClient(202, []byte(`{}`))
	client.agentKey = "agent-key"

	req, err := http.NewRequest("POST", "http://test.com/deltas", nil)
	c.Assert(err, IsNil)
	client.Do(req)

	c.Assert(req.Header.Get(backendhttp.EntityKeyHeader), Equals, client.agentKey)
}

func (*IngestAPISuite) TestPostDeltasGoldenPath(c *C) {
	client := setupMockedClient(202, []byte(`{}`))

	msg, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})

	c.Assert(err, IsNil)
	c.Assert(msg, IsNil)
}

func (*IngestAPISuite) TestPostDeltasGoldenPathGzip(c *C) {
	accumulatedBatches := make(map[int][]byte)
	// set up test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Header.Get("Content-Encoding"), Equals, "gzip")

		gz, err := gzip.NewReader(r.Body)
		c.Assert(err, IsNil)
		data, err := ioutil.ReadAll(gz)
		c.Assert(err, IsNil)

		accumulatedBatches[len(accumulatedBatches)] = data

		// Fake a successful receipt of the deltas
		w.WriteHeader(202)
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	httpClient := backendhttp.GetHttpClient(1*time.Second, &http.Transport{})

	// create real client using test server's URL (instead of mocked client)
	client, _ := NewIngestClient(ts.URL, "abc", "useragent", 6, "", nil, false, httpClient.Do)

	msg, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})
	c.Assert(err, IsNil)
	c.Assert(msg, IsNil)

	c.Assert(strings.TrimSpace(string(accumulatedBatches[0])), Equals, `{"entityKeys":["MyKey","OtherKey"],"isAgent":true,"deltas":[{"source":"","id":0,"timestamp":0,"diff":null,"full_diff":false}]}`)
}

func (*IngestAPISuite) TestPostDeltasGoldenPathWithDeltaStates(c *C) {
	client := setupMockedClient(202, []byte(`{"payload": {"version": 0, "state_map": {"plugin/test": {"last_stored_id": 4, "send_next_id": 2 }}, "reset": "all"}}`))

	pdr, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})

	c.Assert(err, IsNil)
	c.Assert(pdr, NotNil)
	msg := pdr.StateMap
	c.Assert(msg, HasLen, 1)
	c.Assert(msg, FitsTypeOf, DeltaStateMap{"test": &DeltaState{}})
	c.Assert(*(msg)["plugin/test"], DeepEquals, DeltaState{
		LastStoredID: 4,
		SendNextID:   2,
	})
	c.Assert(pdr.Reset, Equals, ResetAll)
}

func (*IngestAPISuite) TestPostDeltasNon200(c *C) {
	client := setupMockedClient(500, []byte(`bug`))

	msg, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})

	c.Assert(err, ErrorMatches, ".*deltas were not accepted: 500.*bug")
	c.Assert(msg, IsNil)
}

func (*IngestAPISuite) TestPostDeltas429(c *C) {
	client := setupMockedClient(429, []byte(`bug`))

	msg, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})

	c.Assert(err, ErrorMatches, ".*deltas were not accepted: 429.*bug")
	iie, ok := err.(*IngestError)
	c.Assert(ok, Equals, true)
	c.Assert(iie.StatusCode, Equals, 429)
	c.Assert(msg, IsNil)
}

func (*IngestAPISuite) TestPostDeltasEmptyReturnBody(c *C) {
	client := setupMockedClient(202, []byte(``))

	client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})

	// c.Assert(err, ErrorMatches, ".*Unable to parse.*unexpected end of JSON.*")
	// c.Assert(msg, IsNil)
}

func (*IngestAPISuite) TestPostDeltasBadJSON(c *C) {
	client := setupMockedClient(202, []byte(``))
	diff := make(map[string]interface{})
	diff["Test}"] = func() bool { return false }

	_, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{Diff: diff})

	c.Assert(err, ErrorMatches, ".*unsupported type.*")
}

func (*IngestAPISuite) TestPostDeltasDoError(c *C) {
	mock := backendhttp.NewMockTransport()
	mock.HttpLibError = fmt.Errorf("fail do")
	mockClient := &http.Client{Transport: mock}
	client := &IngestClient{
		svcUrl:           "http://test.com",
		licenseKey:       "abc",
		HttpClient:       mockClient.Do,
		userAgent:        "agentsmith",
		CompressionLevel: 0,
	}

	_, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})

	c.Assert(err, ErrorMatches, ".*fail do.*")
}

type BadReader struct{}

func (br BadReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("bad read")
}

func (br BadReader) Close() error {
	return nil
}

func (*IngestAPISuite) TestPostDeltasBadBody(c *C) {
	mock := backendhttp.NewMockTransport()
	mock.AppendWithBody(500, BadReader{})
	mockClient := &http.Client{Transport: mock}
	client := &IngestClient{
		svcUrl:           "http://test.com",
		licenseKey:       "abc",
		HttpClient:       mockClient.Do,
		userAgent:        "agentsmith",
		CompressionLevel: 0,
	}

	_, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, true, &RawDelta{})

	c.Assert(err, ErrorMatches, ".*bad read.*")
}

func (*IngestAPISuite) TestInvalidCompressionLevel(c *C) {
	client, err := NewIngestClient("http://test.com", "abc", "useragent", 17, "", nil, false, backendhttp.NullHttpClient)
	c.Assert(client, IsNil)
	c.Assert(err, ErrorMatches, "gzip: invalid compression level: 17")
}

func (*IngestAPISuite) TestPostDeltasBulk(c *C) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(backendhttp.StandardResponse{
		Payload: []BulkDeltaResponse{
			{
				Error: "test",
			},
			{
				EntityKeys: []string{"one"},
				PostDeltaResponse: PostDeltaResponse{
					Version: 1,
				},
			},
		},
	}); err != nil {
		c.Fatal(err)
	}

	mock := backendhttp.NewMockTransport()
	mock.AppendWithBody(http.StatusAccepted, ioutil.NopCloser(&buf))
	mockClient := &http.Client{Transport: mock}
	cl := &IngestClient{
		svcUrl:           "http://test.com",
		licenseKey:       "abc",
		HttpClient:       mockClient.Do,
		userAgent:        "agentsmith",
		CompressionLevel: 0,
	}
	deltas, err := cl.PostDeltasBulk([]PostDeltaBody{})
	c.Check(err, IsNil)
	c.Check(deltas[0].Error, Equals, "test")
	c.Check(deltas[1].EntityKeys[0], Equals, "one")
	c.Check(deltas[1].Version, Equals, int64(1))
}

func (*IngestAPISuite) TestPostDeltasBulkErr(c *C) {
	mock := backendhttp.NewMockTransport()
	mock.AppendWithBody(http.StatusInternalServerError, BadReader{})
	mockClient := &http.Client{Transport: mock}
	cl := &IngestClient{
		svcUrl:           "http://test.com",
		licenseKey:       "abc",
		HttpClient:       mockClient.Do,
		userAgent:        "agentsmith",
		CompressionLevel: 0,
	}
	_, err := cl.PostDeltasBulk([]PostDeltaBody{})
	c.Check(err, ErrorMatches, "*bad read*")
}

func TestPostDeltas_EntityID(t *testing.T) {
	var testCases = []struct {
		isAgent  bool
		expected string // expected result
	}{
		// We expect that the agent deltas contains the entityId while the remote ones do not.
		{true, `{"entityId":1234,"entityKeys":["MyKey","OtherKey"],"isAgent":true,"deltas":[{"source":"","id":0,"timestamp":0,"diff":null,"full_diff":false}]}`},
		{false, `{"entityKeys":["MyKey","OtherKey"],"isAgent":false,"deltas":[{"source":"","id":0,"timestamp":0,"diff":null,"full_diff":false}]}`},
	}

	for _, testCase := range testCases {
		caseName := fmt.Sprintf("IsAgent %t", testCase.isAgent)

		t.Run(caseName, func(t *testing.T) {

			var actual []byte

			agentIDProvide := func() entity.Identity {
				return entity.Identity{ID: 1234}
			}

			// set up test server
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("Content-Encoding"), "gzip")

				gz, err := gzip.NewReader(r.Body)
				assert.NoError(t, err)

				data, err := ioutil.ReadAll(gz)
				assert.NoError(t, err)

				actual = data

				// Fake a successful receipt of the deltas
				w.WriteHeader(202)
				w.Write([]byte("{}"))
			}))
			defer ts.Close()

			httpClient := backendhttp.GetHttpClient(1*time.Second, &http.Transport{})
			// create real client using test server's URL (instead of mocked client)
			client, _ := NewIngestClient(ts.URL, "abc", "useragent", 6, "", agentIDProvide, true, httpClient.Do)

			msg, err := client.PostDeltas([]string{"MyKey", "OtherKey"}, testCase.isAgent, &RawDelta{})
			assert.NoError(t, err)
			assert.Nil(t, msg)

			assert.Equal(t, strings.TrimSpace(string(actual)), testCase.expected)
		})
	}
}
