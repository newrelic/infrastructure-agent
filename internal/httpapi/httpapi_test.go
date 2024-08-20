// Copyright 2021 NewServer Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//nolint:exhaustruct,noctx
package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	networkHelpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	logHelper "github.com/newrelic/infrastructure-agent/test/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HTTPAPITestSuite struct {
	suite.Suite
}

// nolint:paralleltest
func TestHTTPAPITestSuite(t *testing.T) {
	suite.Run(t, new(HTTPAPITestSuite))
}

func (suite *HTTPAPITestSuite) TestServe_Status() {
	// Given a running HTTP endpoint
	port, err := networkHelpers.TCPPort()
	suite.Require().NoError(err)

	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverOk.Close()

	// And a status reporter monitoring it
	endpoints := []string{serverOk.URL}
	healthEndpoint := serverOk.URL
	logger := log.WithComponent(suite.T().Name())
	timeout := 100 * time.Millisecond
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	emptyEntityKeyProvider := func() string {
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := status.NewReporter(ctx, logger, endpoints, healthEndpoint, timeout, transport, emptyIDProvide, emptyEntityKeyProvider, "user-agent", "agent-key")

	// When agent status API server is ready
	em := &testemit.RecordEmitter{}
	s, err := NewServer(r, em)
	require.NoError(suite.T(), err)
	s.Status.Enable("localhost", port)
	defer cancel()

	go s.Serve(ctx)

	s.waitUntilReady()

	// And a request to the status API is sent
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, statusAPIPath), nil)
	require.NoError(suite.T(), err)
	client := http.Client{}

	res, err := client.Do(req)
	require.NoError(suite.T(), err)
	defer res.Body.Close()

	// Then response contains a report for the monitored endpoint
	require.Equal(suite.T(), http.StatusOK, res.StatusCode)

	var gotReport status.Report
	json.NewDecoder(res.Body).Decode(&gotReport)
	require.Len(suite.T(), gotReport.Checks.Endpoints, 1)
	e := gotReport.Checks.Endpoints[0]
	assert.Empty(suite.T(), e.Error)
	assert.True(suite.T(), e.Reachable)
	assert.Equal(suite.T(), serverOk.URL, e.URL)
	h := gotReport.Checks.Health
	suite.Require().True(h.Healthy)
	suite.Require().Empty(h.Error)
}

func (suite *HTTPAPITestSuite) TestServe_OnlyErrors() {
	// Given a running HTTP endpoint and an errored one (which times out)
	port, err := networkHelpers.TCPPort()
	require.NoError(suite.T(), err)

	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverOk.Close()
	serverTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer serverTimeout.Close()

	// And a status reporter monitoring these endpoints
	endpoints := []string{serverOk.URL, serverTimeout.URL}
	healthEndpoint := serverOk.URL
	logger := log.WithComponent(suite.T().Name())
	timeout := 100 * time.Millisecond
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	emptyEntityKeyProvider := func() string {
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := status.NewReporter(ctx, logger, endpoints, healthEndpoint, timeout, transport, emptyIDProvide, emptyEntityKeyProvider, "user-agent", "agent-key")

	// When agent status API server is ready
	em := &testemit.RecordEmitter{}

	s, err := NewServer(r, em)
	require.NoError(suite.T(), err)
	s.Status.Enable("localhost", port)

	go s.Serve(ctx)

	s.waitUntilReady()

	// And a request to the status API is sent
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, statusOnlyErrorsAPIPath), nil)
	require.NoError(suite.T(), err)
	client := http.Client{}

	res, err := client.Do(req)
	require.NoError(suite.T(), err)
	defer res.Body.Close()

	// Then response contains a report for the monitored endpoint
	require.Equal(suite.T(), http.StatusOK, res.StatusCode)

	var gotReport status.Report
	json.NewDecoder(res.Body).Decode(&gotReport)
	require.Len(suite.T(), gotReport.Checks.Endpoints, 1, "only errored endpoint should be reported")
	e := gotReport.Checks.Endpoints[0]
	assert.NotEmpty(suite.T(), e.Error)
	assert.False(suite.T(), e.Reachable)
	assert.Equal(suite.T(), serverTimeout.URL, e.URL)
}

func (suite *HTTPAPITestSuite) TestServe_Entity() {
	logger := log.WithComponent(suite.T().Name())
	timeout := 100 * time.Millisecond
	transport := &http.Transport{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emptyIDProvide := func() entity.Identity {
		return entity.Identity{}
	}
	fooIDProvide := func() entity.Identity {
		return entity.Identity{
			GUID: "foo",
		}
	}
	emptyEntityKeyProvider := func() string {
		return ""
	}
	tests := []struct {
		name         string
		idProvide    id.Provide
		expectedCode int
	}{
		{"empty id", emptyIDProvide, http.StatusNoContent},
		{"pinned id", fooIDProvide, http.StatusOK},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			// Given a running HTTP endpoint and an errored one (which times out)
			port, err := networkHelpers.TCPPort()
			require.NoError(t, err)

			r := status.NewReporter(ctx, logger, []string{}, "", timeout, transport, tt.idProvide, emptyEntityKeyProvider, "user-agent", "agent-key")
			// When agent status API server is ready
			em := &testemit.RecordEmitter{}
			s, err := NewServer(r, em)
			require.NoError(t, err)
			s.Status.Enable("localhost", port)
			defer cancel()

			go s.Serve(ctx)

			s.waitUntilReady()

			// And a request to the status API is sent
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, statusEntityAPIPath), nil)
			require.NoError(t, err)
			client := http.Client{}

			res, err := client.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			// Then response contains an entity report
			require.Equal(t, tt.expectedCode, res.StatusCode)

			if tt.expectedCode != http.StatusNoContent {
				var gotReport status.ReportEntity
				json.NewDecoder(res.Body).Decode(&gotReport)
				assert.Equal(t, tt.idProvide().GUID.String(), gotReport.GUID)
			}
		})
	}
}

func (suite *HTTPAPITestSuite) TestServe_Health() {
	// Given a running HTTP endpoint
	port, err := networkHelpers.TCPPort()
	suite.Require().NoError(err)
	var requestsDone int

	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestsDone > 0 {
			w.WriteHeader(http.StatusUnauthorized)
		}
		w.WriteHeader(http.StatusOK)
		requestsDone++
	}))
	defer serverOk.Close()

	// And a status reporter monitoring it
	logger := log.WithComponent(suite.T().Name())
	timeout := 100 * time.Millisecond
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	emptyEntityKeyProvider := func() string {
		return ""
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := status.NewReporter(ctx, logger, []string{}, serverOk.URL, timeout, transport, emptyIDProvide, emptyEntityKeyProvider, "user-agent", "agent-key")

	// When agent status API server is ready
	em := &testemit.RecordEmitter{}
	server, err := NewServer(r, em)
	suite.Require().NoError(err)
	server.Status.Enable("localhost", port)

	go server.Serve(ctx)

	server.waitUntilReady()

	tests := []struct {
		name       string
		healthy    bool
		statusCode int
	}{
		{"healthy", true, http.StatusOK},
		{"unhealthy", false, http.StatusInternalServerError},
	}
	for _, testCase := range tests {
		suite.T().Run(testCase.name, func(t *testing.T) {
			// And a request to the status API is sent
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d%s", port, statusHealthAPIPath), nil)
			suite.Require().NoError(err)
			client := http.Client{}

			res, err := client.Do(req)
			suite.Require().NoError(err)
			defer res.Body.Close()

			suite.Require().Equal(testCase.statusCode, res.StatusCode)

			var gotReport status.HealthReport
			_ = json.NewDecoder(res.Body).Decode(&gotReport)
			suite.Require().Equal(testCase.healthy, gotReport.Healthy)
		})
	}
}

func (suite *HTTPAPITestSuite) TestServe_IngestData() {
	port, err := networkHelpers.TCPPort()
	require.NoError(suite.T(), err)

	em := &testemit.RecordEmitter{}
	s, err := NewServer(&noopReporter{}, em)
	require.NoError(suite.T(), err)
	s.Ingest.Enable("localhost", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.Serve(ctx)

	payloadWritten := make(chan struct{})
	go func() {
		s.waitUntilReady()
		client := http.Client{}
		postReq, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d%s", port, ingestAPIPath), bytes.NewReader(fixtures.FooBytes))
		resp, err := client.Do(postReq)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), 20, resp.StatusCode/10, "status code: %v", resp.StatusCode)
		close(payloadWritten)
	}()

	select {
	case <-time.NewTimer(1000 * time.Millisecond).C:
		suite.T().Error("timeout waiting for HTTP request to be submitted")
	case <-payloadWritten:
	}
	suite.T().Log("receiving from integration...\n")
	d, err := em.ReceiveFrom(IntegrationName)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "unique foo", d.DataSet.PluginDataSet.Entity.Name)
}

// nolint:funlen,cyclop
func (suite *HTTPAPITestSuite) TestServe_IngestData_mTLS() {
	cases := []struct {
		name           string
		validateClient bool
		sendCert       bool
		shouldFail     bool
	}{
		{
			name:           "without_client_validation",
			validateClient: false,
		},
		{
			name:           "rejects_unauthenticated_client",
			validateClient: true,
			shouldFail:     true,
		},
		{
			name:           "accepts_valid_client",
			validateClient: true,
			sendCert:       true,
		},
	}

	caCertFile, err := ioutil.ReadFile("testdata/rootCA.pem")
	if err != nil {
		suite.T().Fatalf("internal error: cannot load testdata CA: %v", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCertFile)

	for _, testCase := range cases {
		testCase := testCase
		suite.T().Run(testCase.name, func(t *testing.T) {
			port, err := networkHelpers.TCPPort()
			require.NoError(t, err)

			em := &testemit.RecordEmitter{}
			s, err := NewServer(&noopReporter{}, em)
			require.NoError(t, err)
			s.Ingest.Enable("localhost", port)
			s.Ingest.TLS("testdata/localhost.pem", "testdata/localhost-key.pem")
			if testCase.validateClient {
				s.Ingest.VerifyTLSClient("testdata/rootCA.pem")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go s.Serve(ctx)

			payloadWritten := make(chan struct{})
			go func() {
				s.waitUntilReady()
				if testCase.sendCert {
					// WaitUntilReady() is a no-op when mTLS is enabled, which causes the test to race sometimes.
					// Sleeping one second is a dirty workaround to wait for the server to be ready.
					time.Sleep(1 * time.Second)
				}

				client := http.Client{}
				transport := &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: certPool,
					},
				}

				if testCase.sendCert {
					cert, err := tls.LoadX509KeyPair("testdata/client-client.pem", "testdata/client-client-key.pem")
					if err != nil {
						// We cannot suite.T().Fatal if we're not the main goroutine of the test.
						t.Logf("internal error: loading testdata certs: %v", err)
						t.Fail()
						return
					}

					transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
				}

				client.Transport = transport

				postReq, err := http.NewRequest("POST", fmt.Sprintf("https://localhost:%d%s", port, ingestAPIPath), bytes.NewReader(fixtures.FooBytes))
				resp, err := client.Do(postReq)
				if testCase.shouldFail {
					// If we are expecting this request to fail, we won't check for errors.
					return
				}
				require.NoError(t, err)
				require.Equal(t, 20, resp.StatusCode/10, "status code: %v", resp.StatusCode)
				close(payloadWritten)
			}()

			select {
			case <-time.NewTimer(2 * time.Second).C:
				if testCase.shouldFail {
					// Payload not received and test should fail, return.
					return
				}

				t.Fatal("timeout waiting for HTTP request to be submitted")
			case <-payloadWritten:
			}
			t.Log("receiving from integration...\n")
			d, err := em.ReceiveFrom(IntegrationName)
			require.NoError(t, err)
			assert.Equal(t, "unique foo", d.DataSet.PluginDataSet.Entity.Name)
		})
	}
}

func (suite *HTTPAPITestSuite) TestServer_ServeShouldEndSyncrhonouslyIfDisabled() {
	em := &testemit.RecordEmitter{}
	srv, err := NewServer(&noopReporter{}, em)
	require.NoError(suite.T(), err)
	srv.Ingest.enabled = false
	srv.Status.enabled = false

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	timer := time.NewTimer(500 * time.Millisecond)

	go func() {
		<-timer.C
		suite.T().Fatal("there should not be time for this to be executed") // nolint:staticcheck
	}()

	srv.Serve(ctx)
	timer.Stop()
}

func (suite *HTTPAPITestSuite) Test_waitUntilReadyOrError_ShouldEndInCaseOfNoErrorButNotSuccess() {
	em := &testemit.RecordEmitter{}
	srv, err := NewServer(&noopReporter{}, em)
	require.NoError(suite.T(), err)

	srv.timeout = time.Millisecond * 200

	errCh := make(chan error, 1)
	errCh <- nil
	close(errCh)
	err = srv.waitUntilReadyOrError("localhost", "/path", false, false, errCh)
	assert.ErrorIs(suite.T(), err, ErrURLUnreachable)
}

func (suite *HTTPAPITestSuite) TestServer_ServerErrorsShouldBeLogged() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	port, err := networkHelpers.TCPPort()
	require.NoError(suite.T(), err)

	server := &http.Server{Addr: fmt.Sprintf("localhost:%d", port), Handler: nil}
	defer server.Shutdown(ctx) // nolint:errcheck

	go server.ListenAndServe() // nolint:errcheck
	time.Sleep(time.Second * 2)

	em := &testemit.RecordEmitter{}
	srv, err := NewServer(&noopReporter{}, em)
	require.NoError(suite.T(), err)
	srv.Ingest.Enable("localhost", port)
	srv.Status.Enable("localhost", port)

	hook := logHelper.NewInMemoryEntriesHook([]logrus.Level{logrus.FatalLevel, logrus.ErrorLevel})
	log.AddHook(hook)

	srv.Serve(ctx)

	var opError *net.OpError
	expectedEntries := []logrus.Fields{
		{"error": &opError, "component": "api", "message": "Ingest server error"},
		{"error": &opError, "component": "api", "message": "error serving agent ingest"},
		{"error": &opError, "component": "api", "message": "error serving agent status"},
		{"error": &opError, "component": "api", "message": "unable to start Status-API"},
	}
	entries := hook.GetEntries()
	assert.Len(suite.T(), entries, len(expectedEntries))

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Message < entries[j].Message
	})

	for i, entry := range entries {
		assert.Equal(suite.T(), expectedEntries[i]["message"], entry.Message)
		assert.Equal(suite.T(), expectedEntries[i]["component"], entry.Data["component"])
		assert.ErrorAs(suite.T(), entry.Data["error"].(error), expectedEntries[i]["error"]) // nolint:forcetypeassert
	}
}

type noopReporter struct{}

func (r *noopReporter) Report() (status.Report, error) {
	return status.Report{}, nil
}

func (r *noopReporter) ReportErrors() (status.Report, error) {
	return status.Report{}, nil
}

func (r *noopReporter) ReportEntity() (re status.ReportEntity, err error) {
	return status.ReportEntity{}, nil
}

func (r *noopReporter) ReportHealth() status.HealthReport {
	return status.HealthReport{}
}
