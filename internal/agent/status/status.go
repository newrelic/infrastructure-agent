// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package status

import (
	"context"
	"fmt"
	"net/http"
	"sync"
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
//   - backend endpoints reachability statuses
//   - backend communication healthiness
//
// - configuration
// fields will be empty when ReportErrors() report no errors.
type Report struct {
	Checks *ChecksReport `json:"checks,omitempty"`
	Config *ConfigReport `json:"config,omitempty"`
}

type ChecksReport struct {
	Endpoints []EndpointReport `json:"endpoints,omitempty"`
	Health    HealthReport     `json:"health,omitempty"`
}

// ConfigReport configuration used for status report.
type ConfigReport struct {
	ReachabilityTimeout string `json:"reachability_timeout,omitempty"`
}

// EndpointReport represents a single backend endpoint reachability status.
type EndpointReport struct {
	URL       string `json:"url"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

// HealthReport represents the backend communication healthiness status.
type HealthReport struct {
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

// ReportEntity agent entity report.
type ReportEntity struct {
	GUID string `json:"guid"`
	Key  string `json:"key"`
}

// Reporter reports agent status.
type Reporter interface {
	// Report full status report.
	Report() (Report, error)
	// ReportErrors only reports errors found.
	ReportErrors() (Report, error)
	// ReportEntity agent entity report.
	ReportEntity() (ReportEntity, error)
	// ReportHealth agent healthy report.
	ReportHealth() HealthReport
}

type nrReporter struct {
	ctx                    context.Context
	log                    log.Entry
	endpoints              []string // NR backend URLs
	healthEndpoint         string   // NR command backend URL to check communication healthiness
	license                string
	userAgent              string
	idProvide              id.Provide
	agentEntityKeyProvider func() string
	timeout                time.Duration
	transport              http.RoundTripper
}

// Report reports agent status.
func (r *nrReporter) Report() (report Report, err error) {
	return r.report(false)
}

// ReportErrors only reports agent errored state, Report.Checks should be empty when no errors.
func (r *nrReporter) ReportErrors() (report Report, err error) {
	return r.report(true)
}

func (r *nrReporter) report(onlyErrors bool) (report Report, err error) {
	agentID := r.idProvide().ID.String()

	wg := sync.WaitGroup{}
	wg.Add(len(r.endpoints))
	eReportsC := make(chan EndpointReport, len(r.endpoints))

	for _, ep := range r.endpoints {
		go func(endpoint string) {
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
			e := EndpointReport{
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
			eReportsC <- e
			wg.Done()
		}(ep)
	}

	hReportC := make(chan HealthReport, 1)

	wg.Add(1)

	go func() {
		hReportC <- r.getHealth(agentID)

		wg.Done()
	}()

	wg.Wait()
	close(eReportsC)
	close(hReportC)

	var errored bool
	var eReports []EndpointReport
	for e := range eReportsC {
		if !onlyErrors || !e.Reachable {
			eReports = append(eReports, e)
		}
		if !e.Reachable {
			errored = true
		}
	}
	hreport := <-hReportC

	if !onlyErrors || errored {
		if report.Checks == nil {
			report.Checks = &ChecksReport{}
		}
		report.Checks.Endpoints = eReports
		report.Checks.Health = hreport
		report.Config = &ConfigReport{
			ReachabilityTimeout: r.timeout.String(),
		}
	}

	return
}

func (r *nrReporter) ReportEntity() (re ReportEntity, err error) {
	return ReportEntity{
		GUID: r.idProvide().GUID.String(),
		Key:  r.agentEntityKeyProvider(),
	}, nil
}

func (r *nrReporter) ReportHealth() HealthReport {
	agentID := r.idProvide().ID.String()

	return r.getHealth(agentID)
}

// Make a http req to the command api to validate the ingest key is valid and connectivity is ok.
func (r *nrReporter) getHealth(agentID string) HealthReport {
	health, err := backendhttp.CheckEndpointHealthiness(
		r.ctx,
		r.healthEndpoint,
		r.license,
		r.userAgent,
		agentID,
		r.timeout,
		r.transport,
	)

	healthReport := HealthReport{
		Healthy: health,
		Error:   "",
	}
	if err != nil {
		healthReport.Error = err.Error()
	}

	return healthReport
}

// NewReporter creates a new status reporter.
func NewReporter(
	ctx context.Context,
	l log.Entry,
	backendEndpoints []string,
	healthEndpoint string,
	timeout time.Duration,
	transport http.RoundTripper,
	agentIDProvide id.Provide,
	agentEntityKeyProvider func() string,
	license,
	userAgent string,
) Reporter {
	return &nrReporter{
		ctx:                    ctx,
		log:                    l,
		endpoints:              backendEndpoints,
		healthEndpoint:         healthEndpoint,
		license:                license,
		userAgent:              userAgent,
		idProvide:              agentIDProvide,
		agentEntityKeyProvider: agentEntityKeyProvider,
		timeout:                timeout,
		transport:              transport,
	}
}
