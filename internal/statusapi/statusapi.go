// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package statusapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	IntegrationName         = "status-api"
	statusAPIPath           = "/v1/status"
	statusOnlyErrorsAPIPath = "/v1/status/errors"
)

type responseError struct {
	Error string `json:"error"`
}

// Server runtime for status API server.
type Server struct {
	host     string
	port     int
	reporter status.Reporter
	logger   log.Entry
	readyCh  chan struct{}
}

// NewServer creates a new status local-only, read-only, API server.
func NewServer(port int, r status.Reporter) *Server {
	return &Server{
		host:     "localhost", // local only API
		port:     port,
		reporter: r,
		logger:   log.WithComponent(IntegrationName).WithField("port", port),
		readyCh:  make(chan struct{}),
	}
}

// Serve serves status API requests.
func (s *Server) Serve(ctx context.Context) {
	router := httprouter.New()
	// read only API
	router.GET(statusAPIPath, s.handle(false))
	router.GET(statusOnlyErrorsAPIPath, s.handle(true))

	close(s.readyCh)
	s.logger.Info("Status server started.")

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", s.host, s.port), router)
	if err != nil {
		s.logger.WithError(err).Error("trying to listen and serve status")
		return
	}
	s.logger.Debug("Status server stopped.")
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
