// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package statusapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	IntegrationName = "api"
	componentName   = IntegrationName
	statusAPIPath   = "/v1/status"
	ingestAPIPath   = "/v1/data"
)

type responseError struct {
	Error string `json:"error"`
}

// Server runtime for status API server.
type Server struct {
	host       string
	port       int
	reporter   status.Reporter
	logger     log.Entry
	definition integration.Definition
	emitter    emitter.Emitter
	readyCh    chan struct{}
}

// NewServer creates a new status local-only, read-only, API server.
// Nice2Have: decouple services into path handlers.
func NewServer(port int, r status.Reporter, em emitter.Emitter) (*Server, error) {
	d, err := integration.NewAPIDefinition(IntegrationName)
	if err != nil {
		return nil, fmt.Errorf("cannot create API definition for HTTP API server, err: %s", err)
	}

	return &Server{
		host:       "localhost", // local only API
		port:       port,
		reporter:   r,
		logger:     log.WithComponent(componentName).WithField("port", port),
		definition: d,
		emitter:    em,
		readyCh:    make(chan struct{}),
	}, nil
}

// Serve serves status API requests.
func (s *Server) Serve(ctx context.Context) {
	router := httprouter.New()
	router.GET(statusAPIPath, s.handleStatus) // read only API
	router.POST(ingestAPIPath, s.handleIngest)

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

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	rep, err := s.reporter.Report()
	if err != nil {
		jerr := json.NewEncoder(w).Encode(responseError{
			Error: fmt.Sprintf("fetching status report: %s", err),
		})
		if jerr != nil {
			s.logger.WithError(jerr).Warn("couldn't encode a failed response")
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jerr := json.NewEncoder(w).Encode(rep)
	if jerr != nil {
		s.logger.WithError(jerr).Warn("couldn't encode status report")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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
