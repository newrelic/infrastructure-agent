// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
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
	cfg        *Config
	reporter   status.Reporter
	logger     log.Entry
	definition integration.Definition
	emitter    emitter.Emitter
	readyCh    chan struct{}
}

// Config HTTP API configuration.
type Config struct {
	Ingest ServerConfig
	Status ServerConfig
}

type ServerConfig struct {
	enabled bool
	address string
	tls     tlsConfig
}

type tlsConfig struct {
	enabled        bool
	validateClient bool
	certPath       string
	keyPath        string
	caPath         string
}

func NewConfig() *Config {
	return &Config{}
}

func (sc *ServerConfig) Enable(host string, port int) {
	sc.enabled = true
	sc.address = net.JoinHostPort(host, fmt.Sprint(port))
}

func (sc *ServerConfig) TLS(certPath, keyPath string) {
	sc.tls.enabled = true
	sc.tls.certPath = certPath
	sc.tls.keyPath = keyPath
}

func (sc *ServerConfig) VerifyTLSClient(caCertPath string) {
	sc.tls.validateClient = true
	sc.tls.caPath = caCertPath
}

// NewServer creates a new API server.
// Nice2Have: decouple services into path handlers.
// Separate HTTP API configs should be deprecated if we want to unify under a single server & port.
func NewServer(c *Config, r status.Reporter, em emitter.Emitter) (*Server, error) {
	d, err := integration.NewAPIDefinition(IntegrationName)
	if err != nil {
		return nil, fmt.Errorf("cannot create API definition for HTTP API server, err: %s", err)
	}

	l := log.WithComponent(componentName).
		WithField("status_enabled", c.Status.enabled).
		WithField("ingest_enabled", c.Ingest.enabled)

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
	if s.cfg.Status.enabled {
		go func() {
			s.logger.WithFields(logrus.Fields{
				"address": s.cfg.Status.address,
			}).Debug("Status API starting listening.")

			router := httprouter.New()
			// read only API
			router.GET(statusAPIPathReady, s.handleReady)
			router.GET(statusEntityAPIPath, s.handleEntity)
			router.GET(statusAPIPath, s.handle(false))
			router.GET(statusOnlyErrorsAPIPath, s.handle(true))
			// local only API
			err := http.ListenAndServe(s.cfg.Status.address, router)
			if err != nil {
				s.logger.WithError(err).Error("unable to start Status-API")
				return
			}
			s.logger.Debug("Status API stopped.")
		}()
	}

	if s.cfg.Ingest.enabled {
		go func() {
			err := s.serveIngest()
			if err != nil {
				log.WithError(err).Error("Ingest server error")
			}
		}()
	}

	c := http.Client{}
	var ingestReady, statusReady bool
	for {
		if !ingestReady && s.cfg.Ingest.enabled {
			scheme := "http://"
			if s.cfg.Ingest.tls.enabled {
				scheme = "https://"
			}
			ingestReady = s.isGetSuccessful(c, scheme+s.cfg.Ingest.address+ingestAPIPathReady)
		}
		if !statusReady && s.cfg.Status.enabled {
			scheme := "http://"
			if s.cfg.Ingest.tls.enabled {
				scheme = "https://"
			}

			statusReady = s.isGetSuccessful(c, scheme+s.cfg.Status.address+statusAPIPathReady)
		}

		if s.allReadyOrDisabled(ingestReady, statusReady) {
			break
		}
		time.Sleep(readinessProbeRetryBackoff)
	}
	close(s.readyCh)

	<-ctx.Done()
}

// serveIngest creates and starts an HTTP server handling ingestAPIPathReady and ingestAPIPath using Config.Ingest
func (s *Server) serveIngest() error {
	s.logger.WithFields(logrus.Fields{
		"address": s.cfg.Ingest.address,
	}).Debug("Ingest API starting listening.")

	router := httprouter.New()
	router.GET(ingestAPIPathReady, s.handleReady)
	router.POST(ingestAPIPath, s.handleIngest)

	server := &http.Server{
		Handler: router,
		Addr:    s.cfg.Ingest.address,
	}

	if s.cfg.Ingest.tls.enabled {
		if s.cfg.Ingest.tls.validateClient {
			err := addMTLS(server, s.cfg.Ingest.tls.caPath)
			if err != nil {
				return fmt.Errorf("creating mTLS server: %w", err)
			}
		}

		s.logger.Debug("starting tls server")
		err := server.ListenAndServeTLS(s.cfg.Ingest.tls.certPath, s.cfg.Ingest.tls.keyPath)
		if err != nil {
			return fmt.Errorf("starting tls server: %w", err)
		}
	}

	err := server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("starting Ingest server: %w", err)
	}

	return errors.New("Ingest server stopped")
}

func (s *Server) allReadyOrDisabled(ingestReady, statusReady bool) bool {
	if s.cfg.Ingest.enabled && !ingestReady {
		return false
	}
	if s.cfg.Status.enabled && !statusReady {
		return false
	}
	return true
}

func (s *Server) isGetSuccessful(c http.Client, URL string) bool {
	postReq, err := http.NewRequest(http.MethodGet, URL, bytes.NewReader([]byte{}))
	if err != nil {
		s.logger.Warnf("cannot create request for %s, error: %s", URL, err)
		return false
	}
	resp, err := c.Do(postReq)
	if err != nil {
		return false
	}

	// Hack: If URL is HTTPs, readiness probe should succeed even if mTLS verification fails
	if strings.HasPrefix(URL, "https") {
		return true
	}

	return resp.StatusCode == http.StatusOK
}

// WaitUntilReady blocks the call until server is ready to accept connections.
func (s *Server) WaitUntilReady() {
	_, _ = <-s.readyCh
}

// handle returns a HTTP handler function for full Status report or just errors Status report.
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
				Error: fmt.Sprintf("fetching Status report: %s", err),
			})
			if jerr != nil {
				s.logger.WithError(jerr).Warn("couldn't encode a failed response")
			}
			return
		}

		b, jerr := json.Marshal(rep)
		if jerr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.WithError(jerr).Warn("couldn't encode Status report")
			return
		}

		if rep.Checks == nil {
			w.WriteHeader(http.StatusCreated) // 201
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		_, err = w.Write(b)
		if err != nil {
			s.logger.Warn("cannot write Status response, error: " + err.Error())
		}
	}
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	re, err := s.reporter.ReportEntity()
	if err != nil {
		s.logger.WithError(err).Error("cannot report entity Status")
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
