package status

import (
	"context"
	"fmt"
	"net/http"
	"time"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	endpointTimeoutMsg = "endpoint check timeout exceeded"
)
type Report struct {
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	URL       string `json:"url"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

type Reporter interface {
	Report() (Report, error)
}

type nrReporter struct {
	ctx       context.Context
	log       log.Entry
	endpoints []string // NR backend URLs
	license   string
	userAgent string
	agentKey  string
	timeout   time.Duration
	transport http.RoundTripper
}

func (r *nrReporter) Report() (report Report, err error) {
	for _, endpoint := range r.endpoints {
		timedout, err := backendhttp.CheckEndpointAvailability(r.ctx, r.log, endpoint, r.license, r.userAgent, r.agentKey, r.timeout, r.transport)
		e := Endpoint{
			URL:       endpoint,
			Reachable: true,
		}
		if timedout || err != nil {
			e.Reachable = false
			if timedout {
				e.Error = fmt.Sprintf("%s, %s", endpointTimeoutMsg ,err)
			} else {
				e.Error = err.Error()
			}
		}

		report.Endpoints = append(report.Endpoints, e)
	}

	return
}

func NewReporter(
	ctx context.Context,
	l log.Entry,
	backendEndpoints []string,
	timeout time.Duration,
	transport http.RoundTripper,
	license,
	userAgent,
	agentKey string,
) Reporter {

	return &nrReporter{
		ctx:       ctx,
		log:       l,
		endpoints: backendEndpoints,
		license:   license,
		userAgent: userAgent,
		agentKey:  agentKey,
		timeout:   timeout,
		transport: transport,
	}
}
