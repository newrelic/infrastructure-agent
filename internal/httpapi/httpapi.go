// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
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
	statusEntityAPIPath        = "/v1/status/entity"
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
	Ingest struct {
		Enabled bool
		Address string
		TLS     struct {
			Enabled  bool
			CertPath string
			KeyPath  string
			CAPath   string
		}
	}
	Status struct {
		Enabled bool
		Address string
	}
}

// NewConfig creates a new API config.
func NewConfig(enableIngest bool, hostIngest string, portIngest int, enableStatus bool, portStatus int) Config {
	c := Config{}
	c.Ingest.Enabled = enableIngest
	c.Ingest.Address = net.JoinHostPort(hostIngest, fmt.Sprint(portIngest))
	c.Status.Enabled = enableStatus
	c.Status.Address = net.JoinHostPort("localhost", fmt.Sprint(portIngest))
	return c
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
		WithField("status_enabled", c.Status.Enabled).
		WithField("ingest_enabled", c.Ingest.Enabled)

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
	if s.cfg.Status.Enabled {
		go func() {
			s.logger.WithFields(logrus.Fields{
				"address": s.cfg.Status.Address,
			}).Debug("Status API starting listening.")

			router := httprouter.New()
			// read only API
			router.GET(statusAPIPathReady, s.handleReady)
			router.GET(statusEntityAPIPath, s.handleEntity)
			router.GET(statusAPIPath, s.handle(false))
			router.GET(statusOnlyErrorsAPIPath, s.handle(true))
			// local only API
			err := http.ListenAndServe(s.cfg.Status.Address, router)
			if err != nil {
				s.logger.WithError(err).Error("unable to start Status-API")
				return
			}
			s.logger.Debug("Status API stopped.")
		}()
	}

	if s.cfg.Ingest.Enabled {
		go func() {
			s.logger.WithFields(logrus.Fields{
				"address": s.cfg.Ingest.Address,
			}).Debug("Ingest API starting listening.")

			router := httprouter.New()
			router.GET(ingestAPIPathReady, s.handleReady)
			router.POST(ingestAPIPath, s.handleIngest)

			if s.cfg.Ingest.TLS.Enabled {
				server, err := tlsServer(s.cfg.Ingest.Address, s.cfg.Ingest.TLS.CAPath, router)
				if err != nil {
					s.logger.WithError(err).Error("cannot create https server")
					return
				}

				s.logger.Debug("starting TLS server")
				err = server.ListenAndServeTLS(s.cfg.Ingest.TLS.CertPath, s.cfg.Ingest.TLS.KeyPath)
				if err != nil {
					s.logger.WithError(err).Error("cannot start https server")
					return
				}

				return
			}

			err := http.ListenAndServe(s.cfg.Ingest.Address, router)
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
		if !ingestReady && s.cfg.Ingest.Enabled {
			ingestReady = s.isGetSuccessful(c, s.cfg.Ingest.Address+ingestAPIPathReady)
		}
		if !statusReady && s.cfg.Status.Enabled {
			statusReady = s.isGetSuccessful(c, s.cfg.Status.Address+statusAPIPathReady)
		}

		if s.allReadyOrDisabled(ingestReady, statusReady) {
			break
		}
		time.Sleep(readinessProbeRetryBackoff)
	}
	close(s.readyCh)

	<-ctx.Done()
}

func (s *Server) allReadyOrDisabled(ingestReady, statusReady bool) bool {
	if s.cfg.Ingest.Enabled && !ingestReady {
		return false
	}
	if s.cfg.Status.Enabled && !statusReady {
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

func (s *Server) handleEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	re, err := s.reporter.ReportEntity()
	if err != nil {
		s.logger.WithError(err).Error("cannot report entity status")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if re.GUID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	b, jerr := json.Marshal(re)
	if jerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.WithError(jerr).Warn("couldn't encode entity report")
		return
	}

	_, err = w.Write(b)
	if err != nil {
		s.logger.Warn("cannot write entity response, error: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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
