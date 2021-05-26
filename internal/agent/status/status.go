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
// - checks:
//   * backend endpoints reachability statuses
// - configuration
// fields will be empty when ReportErrors() report no errors.
type Report struct {
	Data   Data         `json:"data,omitempty"`
	Config ConfigReport `json:"config,omitempty"`
}

type Data struct {
	Endpoints []Endpoint `json:"endpoints"`
}

// ConfigReport configuration used for status report.
type ConfigReport struct {
	ReachabilityTimeout string `json:"reachability_timeout"`
}

// Endpoint represents a single backend endpoint reachability status.
type Endpoint struct {
	URL       string `json:"url"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

// Reporter reports agent status.
type Reporter interface {
	// Report full status report.
	Report() (Report, error)
	// ReportErrors only reports errors found.
	ReportErrors() (Report, error)
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
	return r.report(false)
}

// ReportErrors only reports agent errored state, Report.Data should be empty when no errors.
func (r *nrReporter) ReportErrors() (report Report, err error) {
	return r.report(true)
}

func (r *nrReporter) report(onlyErrors bool) (report Report, err error) {
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
		errored := timedout || err != nil
		if errored {
			e.Reachable = false
			if timedout {
				e.Error = fmt.Sprintf("%s, %s", endpointTimeoutMsg, err)
			} else {
				e.Error = err.Error()
			}
		}

		if !onlyErrors || errored {
			report.Data.Endpoints = append(report.Data.Endpoints, e)
			report.Config = ConfigReport{
				ReachabilityTimeout: r.timeout.String(),
			}

		}
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
