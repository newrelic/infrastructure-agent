package status

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestReporterI_Report(t *testing.T) {
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

	expectReportOk := Report{Endpoints: []Endpoint{{
		URL:       serverOk.URL,
		Reachable: true,
	}}}
	expectReportTimeout := Report{Endpoints: []Endpoint{{
		URL:       serverTimeout.URL,
		Reachable: false,
		Error:     endpointTimeoutMsg, // substring is enough, it'll assert via "string contains"
	}}}
	expectReportMixed := Report{Endpoints: []Endpoint{
		{
			URL:       serverOk.URL,
			Reachable: true,
		},
		{
			URL:       serverTimeout.URL,
			Reachable: false,
			Error:     endpointTimeoutMsg,
		},
	}}

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
			timeout := 10 * time.Millisecond
			transport := &http.Transport{}
			r := NewReporter(context.Background(), l, tt.endpoints, timeout, transport, "license", "user-agent", "agent-key")

			got, err := r.Report()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			for i, expectedEndpoint := range tt.want.Endpoints {
				gotEndpoint := got.Endpoints[i]
				assert.Equal(t, expectedEndpoint.URL, gotEndpoint.URL)
				assert.Equal(t, expectedEndpoint.Reachable, gotEndpoint.Reachable)
				assert.Contains(t, gotEndpoint.Error, expectedEndpoint.Error)
			}
		})
	}
}
