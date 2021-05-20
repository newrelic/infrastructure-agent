// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package status

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	endpointTimeoutMsg = "endpoint check timeout exceeded"
)

// Report agent status report. It contains:
// - backend endpoints reachability statuses
type Report struct {
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint represents a single backend endpoint reachability status.
type Endpoint struct {
	URL       string `json:"url"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

// Reporter reports agent status.
type Reporter interface {
	Report() (Report, error)
}

type nrReporter struct {
	ctx       context.Context
	log       log.Entry
	endpoints []string // NR backend URLs
	license   string
	userAgent string
	idProvide id.Provide
	timeout   time.Duration
	transport http.RoundTripper
}

// Report reports agent status.
func (r *nrReporter) Report() (report Report, err error) {
	agentID := r.idProvide().ID.String()
	for _, endpoint := range r.endpoints {
		timedout, err := backendhttp.CheckEndpointReachability(
			r.ctx,
			r.log,
			endpoint,
			r.license,
			r.userAgent,
			agentID,
			r.timeout,
			r.transport,
		)
		e := Endpoint{
			URL:       endpoint,
			Reachable: true,
		}
		if timedout || err != nil {
			e.Reachable = false
			if timedout {
				e.Error = fmt.Sprintf("%s, %s", endpointTimeoutMsg, err)
			} else {
				e.Error = err.Error()
			}
		}

		report.Endpoints = append(report.Endpoints, e)
	}

	return
}

// NewReporter creates a new status reporter.
func NewReporter(
	ctx context.Context,
	l log.Entry,
	backendEndpoints []string,
	timeout time.Duration,
	transport http.RoundTripper,
	agentIDProvide id.Provide,
	license,
	userAgent string,
) Reporter {

	return &nrReporter{
		ctx:       ctx,
		log:       l,
		endpoints: backendEndpoints,
		license:   license,
		userAgent: userAgent,
		idProvide: agentIDProvide,
		timeout:   timeout,
		transport: transport,
	}
}
