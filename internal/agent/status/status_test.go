// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//nolint:exhaustruct,noctx
package status

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReporter_Report(t *testing.T) {
	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverOk.Close()

	serverTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Reduced timeout time to make the test faster
	}))
	defer serverTimeout.Close()

	serverUnauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer serverUnauthorized.Close()

	assert.Eventually(t,
		func() bool {
			res, err := serverOk.Client().Get(serverOk.URL)

			return err == nil && res.StatusCode == http.StatusOK
		},
		time.Second, 10*time.Millisecond)

	endpointsOk := []string{serverOk.URL}
	healthEndpointOK := serverOk.URL
	endpointsTimeout := []string{serverTimeout.URL}
	healthEndpointTimeout := serverTimeout.URL
	endpointsMixed := []string{serverOk.URL, serverTimeout.URL}
	healthEndpointUnauthorized := serverUnauthorized.URL

	expectReportOk := Report{Checks: &ChecksReport{
		Endpoints: []EndpointReport{
			{
				URL:       serverOk.URL,
				Reachable: true,
				Error:     "",
			},
		},
		Health: HealthReport{
			Healthy: true,
			Error:   "",
		},
	}, Config: nil}

	expectReportTimeout := Report{Checks: &ChecksReport{
		Endpoints: []EndpointReport{
			{
				URL:       serverTimeout.URL,
				Reachable: false,
				Error:     endpointTimeoutMsg, // substring is enough, it'll assert via "string contains"
			},
		},
		Health: HealthReport{
			Healthy: false,
			Error:   "context deadline exceeded",
		},
	}, Config: nil}

	expectReportMixed := Report{Checks: &ChecksReport{
		Endpoints: []EndpointReport{
			{
				URL:       serverOk.URL,
				Reachable: true,
				Error:     "",
			},
			{
				URL:       serverTimeout.URL,
				Reachable: false,
				Error:     endpointTimeoutMsg,
			},
		},
		Health: HealthReport{
			Healthy: false,
			Error:   http2.ErrUnexepectedResponseCode.Error(),
		},
	}, Config: nil}

	timeout := 500 * time.Millisecond // Increased timeout to account for system delays
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	emptyEntityKeyProvider := func() string {
		return ""
	}
	tests := []struct {
		name           string
		endpoints      []string
		healthEndpoint string
		want           Report
		wantErr        bool
	}{
		{"connectivity ok", endpointsOk, healthEndpointOK, expectReportOk, false},
		{"connectivity timedout", endpointsTimeout, healthEndpointTimeout, expectReportTimeout, false},
		{"connectivities ok and timeout and unhealthy", endpointsMixed, healthEndpointUnauthorized, expectReportMixed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := log.WithComponent(tt.name)
			r := NewReporter(context.Background(), l, tt.endpoints, tt.healthEndpoint, timeout, transport, emptyIDProvide, emptyEntityKeyProvider, "user-agent", "agent-key")

			got, err := r.Report()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, timeout.String(), got.Config.ReachabilityTimeout)
			for _, expectedEndpoint := range tt.want.Checks.Endpoints {
				found := -1
				for idx, gotEndpoint := range got.Checks.Endpoints {
					if gotEndpoint.URL == expectedEndpoint.URL {
						found = idx
						break
					}
				}
				require.NotEqualf(t, -1, found, "endpoint not found in response: %s", expectedEndpoint.URL)
				gotEndpoint := got.Checks.Endpoints[found]
				assert.Equal(t, expectedEndpoint.URL, gotEndpoint.URL)
				assert.Equal(t, expectedEndpoint.Reachable, gotEndpoint.Reachable)
				assert.Contains(t, gotEndpoint.Error, expectedEndpoint.Error)
			}
			assert.Equal(t, tt.want.Checks.Health.Healthy, got.Checks.Health.Healthy)
			assert.Contains(t, got.Checks.Health.Error, tt.want.Checks.Health.Error)
		})
	}
}

func TestNewReporter_ReportErrors(t *testing.T) {
	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverOk.Close()

	serverTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer serverTimeout.Close()

	endpointsOk := []string{serverOk.URL}
	healthEndpointOK := serverOk.URL
	endpointsTimeout := []string{serverTimeout.URL}
	endpointsMixed := []string{serverOk.URL, serverTimeout.URL}

	expectReportOk := Report{}
	expectReportTimeout := Report{Checks: &ChecksReport{Endpoints: []EndpointReport{{
		URL:       serverTimeout.URL,
		Reachable: false,
		Error:     endpointTimeoutMsg, // substring is enough, it'll assert via "string contains"
	}}}}
	expectReportMixed := Report{Checks: &ChecksReport{Endpoints: []EndpointReport{
		{
			URL:       serverTimeout.URL,
			Reachable: false,
			Error:     endpointTimeoutMsg,
		},
	}}}

	timeout := 10 * time.Millisecond
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	emptyEntityKeyProvider := func() string {
		return ""
	}
	tests := []struct {
		name      string
		endpoints []string
		want      Report
		wantErr   bool
	}{
		{"connectivity ok", endpointsOk, expectReportOk, false},
		{"connectivity timedout", endpointsTimeout, expectReportTimeout, false},
		{"connectivities ok and timeout", endpointsMixed, expectReportMixed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := log.WithComponent(tt.name)
			r := NewReporter(context.Background(), l, tt.endpoints, healthEndpointOK, timeout, transport, emptyIDProvide, emptyEntityKeyProvider, "user-agent", "agent-key")

			got, err := r.ReportErrors()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.want.Checks == nil {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, timeout.String(), got.Config.ReachabilityTimeout)
				require.Len(t, got.Checks.Endpoints, len(tt.want.Checks.Endpoints))
				for _, expectedEndpoint := range tt.want.Checks.Endpoints {
					found := -1
					for idx, gotEndpoint := range got.Checks.Endpoints {
						if gotEndpoint.URL == expectedEndpoint.URL {
							found = idx
							break
						}
					}
					require.NotEqualf(t, -1, found, "endpoint not found in response: %s", expectedEndpoint.URL)
					gotEndpoint := got.Checks.Endpoints[found]
					assert.Equal(t, expectedEndpoint.URL, gotEndpoint.URL)
					assert.Equal(t, expectedEndpoint.Reachable, gotEndpoint.Reachable)
					assert.Contains(t, gotEndpoint.Error, expectedEndpoint.Error)
				}
			}
		})
	}
}

func TestNewReporter_ReportEntity(t *testing.T) {
	timeout := 10 * time.Millisecond
	transport := &http.Transport{}

	tests := []struct {
		name      string
		guid      string
		entityKey string
		want      ReportEntity
		wantErr   bool
	}{
		{"no guid", "", "", ReportEntity{}, false},
		{"foo guid", "foo", "", ReportEntity{GUID: "foo"}, false},
		{"foo guid bar key", "foo", "bar", ReportEntity{GUID: "foo", Key: "bar"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idProvide := func() entity.Identity {
				return entity.Identity{
					GUID: entity.GUID(tt.guid),
				}
			}
			l := log.WithComponent(tt.name)
			entityKeyProvider := func() string {
				return tt.entityKey
			}
			r := NewReporter(context.Background(), l, []string{}, "", timeout, transport, idProvide, entityKeyProvider, "user-agent", "agent-key")

			got, err := r.ReportEntity()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.guid, got.GUID)
		})
	}
}

//nolint:paralleltest
func TestNewReporter_ReportHealth(t *testing.T) {
	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverOk.Close()

	serverTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer serverTimeout.Close()

	serverUnauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer serverUnauthorized.Close()

	assert.Eventually(t,
		func() bool {
			res, err := serverOk.Client().Get(serverOk.URL)
			defer func() {
				_ = res.Body.Close()
			}()

			return err == nil && res.StatusCode == 200
		},
		time.Second, 10*time.Millisecond)

	healthEndpointOK := serverOk.URL
	healthEndpointTimeout := serverTimeout.URL
	healthEndpointUnauthorized := serverUnauthorized.URL

	expectReportOk := HealthReport{
		Healthy: true,
		Error:   "",
	}

	expectReportTimeout := HealthReport{
		Healthy: false,
		Error:   "context deadline exceeded",
	}

	expectReportUnauthorized := HealthReport{
		Healthy: false,
		Error:   http2.ErrUnexepectedResponseCode.Error(),
	}

	timeout := 10 * time.Millisecond
	transport := &http.Transport{}
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}
	emptyEntityKeyProvider := func() string {
		return ""
	}
	tests := []struct {
		name           string
		healthEndpoint string
		want           HealthReport
	}{
		{"connectivity ok", healthEndpointOK, expectReportOk},
		{"connectivity timedout", healthEndpointTimeout, expectReportTimeout},
		{"unhealthy", healthEndpointUnauthorized, expectReportUnauthorized},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			l := log.WithComponent(testCase.name)
			r := NewReporter(context.Background(), l, nil, testCase.healthEndpoint, timeout, transport, emptyIDProvide, emptyEntityKeyProvider, "user-agent", "agent-key")

			got := r.ReportHealth()

			assert.Equal(t, testCase.want.Healthy, got.Healthy)
			assert.Contains(t, got.Error, testCase.want.Error)
		})
	}
}
