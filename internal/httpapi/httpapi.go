// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

const (
	IntegrationName            = "api"
	componentName              = IntegrationName
	statusAPIPath              = "/v1/status"
	statusOnlyErrorsAPIPath    = "/v1/status/errors"
	statusAPIPathReady         = "/v1/status/ready"
	ingestAPIPath              = "/v1/data"
	ingestAPIPathReady         = "/v1/data/ready"
	readinessProbeRetryBackoff = 10 * time.Millisecond
)

type responseError struct {
	Error string `json:"error"`
}

// Server runtime for status API server.
type Server struct {
	cfg        Config
	reporter   status.Reporter
	logger     log.Entry
	definition integration.Definition
	emitter    emitter.Emitter
	readyCh    chan struct{}
}

// Config HTTP API configuration.
type Config struct {
	EnableIngest bool
	EnableStatus bool
	PortIngest   int
	HostIngest   string
	PortStatus   int
}

// NewConfig creates a new API config.
func NewConfig(enableIngest bool, hostIngest string, portIngest int, enableStatus bool, portStatus int) Config {
	return Config{
		EnableIngest: enableIngest,
		EnableStatus: enableStatus,
		PortIngest:   portIngest,
		PortStatus:   portStatus,
		HostIngest:   hostIngest,
	}
}

// NewServer creates a new API server.
// Nice2Have: decouple services into path handlers.
// Separate HTTP API configs should be deprecated if we want to unify under a single server & port.
func NewServer(c Config, r status.Reporter, em emitter.Emitter) (*Server, error) {
	d, err := integration.NewAPIDefinition(IntegrationName)
	if err != nil {
		return nil, fmt.Errorf("cannot create API definition for HTTP API server, err: %s", err)
	}

	l := log.WithComponent(componentName).
		WithField("status_enabled", c.EnableStatus).
		WithField("status_enabled", c.EnableIngest)

	return &Server{
		cfg:        c,
		reporter:   r,
		logger:     l,
		definition: d,
		emitter:    em,
		readyCh:    make(chan struct{}),
	}, nil
}

// Serve serves status API requests.
// Nice2Have: context cancellation.
func (s *Server) Serve(ctx context.Context) {
	if s.cfg.EnableStatus {
		go func() {
			s.logger.WithFields(logrus.Fields{
				"port": s.cfg.PortStatus,
			}).Debug("Status API starting listening.")

			router := httprouter.New()
			// read only API
			router.GET(statusAPIPathReady, s.handleReady)
			router.GET(statusAPIPath, s.handle(false))
			router.GET(statusOnlyErrorsAPIPath, s.handle(true))
			// local only API
			err := http.ListenAndServe(fmt.Sprintf("%s:%d", "localhost", s.cfg.PortStatus), router)
			if err != nil {
				s.logger.WithError(err).Error("unable to start Status-API")
				return
			}
			s.logger.Debug("Status API stopped.")
		}()
	}

	if s.cfg.EnableIngest {
		go func() {
			s.logger.WithFields(logrus.Fields{
				"port": s.cfg.PortIngest,
				"host": s.cfg.HostIngest,
			}).Debug("Ingest API starting listening.")

			router := httprouter.New()
			router.GET(ingestAPIPathReady, s.handleReady)
			router.POST(ingestAPIPath, s.handleIngest)
			err := http.ListenAndServe(fmt.Sprintf("%s:%d", s.cfg.HostIngest, s.cfg.PortIngest), router)
			if err != nil {
				s.logger.WithError(err).Error("unable to start Ingest-API")
				return
			}
			s.logger.Debug("Ingest API stopped.")
		}()
	}

	c := http.Client{}
	var ingestReady, statusReady bool
	for {
		if !ingestReady && s.cfg.EnableIngest {
			ingestReady = s.isGetSuccessful(c, fmt.Sprintf("http://%s:%d%s", s.cfg.HostIngest, s.cfg.PortIngest, ingestAPIPathReady))
		}
		if !statusReady && s.cfg.EnableStatus {
			statusReady = s.isGetSuccessful(c, fmt.Sprintf("http://localhost:%d%s", s.cfg.PortStatus, statusAPIPathReady))
		}

		if s.allReadyOrDisabled(ingestReady, statusReady) {
			break
		}
		time.Sleep(readinessProbeRetryBackoff)
	}
	close(s.readyCh)

	<-ctx.Done()
}

func (s *Server) allReadyOrDisabled(ingestReadyOrDisabled, statusReadyOrDisabled bool) bool {
	if !s.cfg.EnableIngest && !s.cfg.EnableStatus {
		return true
	}
	if s.cfg.EnableIngest && !ingestReadyOrDisabled {
		return false
	}
	if s.cfg.EnableStatus && !statusReadyOrDisabled {
		return false
	}
	return true
}

func (s *Server) isGetSuccessful(c http.Client, URL string) bool {
	postReq, err := http.NewRequest(http.MethodGet, URL, bytes.NewReader([]byte{}))
	if err != nil {
		s.logger.Warnf("cannot create request for %s, error: %s", URL, err)
	} else if resp, err := c.Do(postReq); err == nil && resp.StatusCode == http.StatusOK {
		return true
	}

	return false
}

// WaitUntilReady blocks the call until server is ready to accept connections.
func (s *Server) WaitUntilReady() {
	_, _ = <-s.readyCh
}

// handle returns a HTTP handler function for full status report or just errors status report.
func (s *Server) handle(onlyErrors bool) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		var rep status.Report
		var err error
		if onlyErrors {
			rep, err = s.reporter.ReportErrors()
		} else {
			rep, err = s.reporter.Report()
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jerr := json.NewEncoder(w).Encode(responseError{
				Error: fmt.Sprintf("fetching status report: %s", err),
			})
			if jerr != nil {
				s.logger.WithError(jerr).Warn("couldn't encode a failed response")
			}
			return
		}

		b, jerr := json.Marshal(rep)
		if jerr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.WithError(jerr).Warn("couldn't encode status report")
			return
		}

		if rep.Checks == nil {
			w.WriteHeader(http.StatusCreated) // 201
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		_, err = w.Write(b)
		if err != nil {
			s.logger.Warn("cannot write status response, error: " + err.Error())
		}
	}
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	rawBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errMsg := "cannot read HTTP payload"
		s.logger.WithError(err).Warn(errMsg)
		w.WriteHeader(http.StatusBadRequest)
		jerr := json.NewEncoder(w).Encode(responseError{
			Error: fmt.Sprintf("%s: %s", errMsg, err.Error()),
		})
		if jerr != nil {
			s.logger.WithError(jerr).Warn("couldn't encode a failed response")
		}
		return
	}

	err = s.emitter.Emit(s.definition, nil, nil, rawBody)
	if err != nil {
		errMsg := "cannot emit HTTP payload"
		s.logger.WithError(err).Warn(errMsg)
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(fmt.Sprintf("%s, err: %s", errMsg, err.Error())))
		if err != nil {
			s.logger.WithError(err).Warn("cannot write HTTP response body")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
