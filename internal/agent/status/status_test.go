// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package status

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReporter_Report(t *testing.T) {
	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer serverOk.Close()
	serverTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer serverTimeout.Close()

	assert.Eventually(t,
		func() bool {
			res, err := serverOk.Client().Get(serverOk.URL)
			return err == nil && res.StatusCode == 200
		},
		time.Second, 10*time.Millisecond)

	endpointsOk := []string{serverOk.URL}
	endpointsTimeout := []string{serverTimeout.URL}
	endpointsMixed := []string{serverOk.URL, serverTimeout.URL}

	expectReportOk := Report{Checks: &ChecksReport{Endpoints: []EndpointReport{{
		URL:       serverOk.URL,
		Reachable: true,
	}}}}
	expectReportTimeout := Report{Checks: &ChecksReport{Endpoints: []EndpointReport{{
		URL:       serverTimeout.URL,
		Reachable: false,
		Error:     endpointTimeoutMsg, // substring is enough, it'll assert via "string contains"
	}}}}
	expectReportMixed := Report{Checks: &ChecksReport{Endpoints: []EndpointReport{
		{
			URL:       serverOk.URL,
			Reachable: true,
		},
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
			r := NewReporter(context.Background(), l, tt.endpoints, timeout, transport, emptyIDProvide, "user-agent", "agent-key")

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
		})
	}
}

func TestNewReporter_ReportErrors(t *testing.T) {
	serverOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer serverOk.Close()
	serverTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer serverTimeout.Close()

	endpointsOk := []string{serverOk.URL}
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
			r := NewReporter(context.Background(), l, tt.endpoints, timeout, transport, emptyIDProvide, "user-agent", "agent-key")

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
		name    string
		guid    string
		want    ReportEntity
		wantErr bool
	}{
		{"no guid", "", ReportEntity{}, false},
		{"foo guid", "foo", ReportEntity{GUID: "foo"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idProvide := func() entity.Identity {
				return entity.Identity{
					GUID: entity.GUID(tt.guid),
				}
			}
			l := log.WithComponent(tt.name)
			r := NewReporter(context.Background(), l, []string{}, timeout, transport, idProvide, "user-agent", "agent-key")

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
