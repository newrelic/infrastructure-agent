// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/emitter"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/integration"
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
	readinessProbeRetryBackoff = 100 * time.Millisecond
)

var ErrServerStopped = errors.New("server stopped")

type responseError struct {
	Error string `json:"error"`
}

// Server runtime for status API server.
type Server struct {
	Ingest     ComponentConfig
	Status     ComponentConfig
	reporter   status.Reporter
	logger     log.Entry
	definition integration.Definition
	emitter    emitter.Emitter
	readyCh    chan struct{}
}

// ComponentConfig stores configuration for a server component.
type ComponentConfig struct {
	enabled bool
	address string
	tls     tlsConfig
}

// tlsConfig stores tls-related configuration.
type tlsConfig struct {
	enabled        bool
	validateClient bool
	certPath       string
	keyPath        string
	caPath         string
}

// Enable configures and enables a server component.
func (sc *ComponentConfig) Enable(host string, port int) {
	sc.enabled = true
	sc.address = net.JoinHostPort(host, fmt.Sprint(port))
}

// TLS configures and enables TLS for a server component.
func (sc *ComponentConfig) TLS(certPath, keyPath string) {
	sc.tls.enabled = true
	sc.tls.certPath = certPath
	sc.tls.keyPath = keyPath
}

// VerifyTLSClient configures and enables TLS client certificate validation for a server component.
func (sc *ComponentConfig) VerifyTLSClient(caCertPath string) {
	sc.tls.validateClient = true
	sc.tls.caPath = caCertPath
}

// NewServer creates a new API server.
// Nice2Have: decouple services into path handlers.
// Separate HTTP API configs should be deprecated if we want to unify under a single server & port.
func NewServer(r status.Reporter, em emitter.Emitter) (*Server, error) {
	d, err := integration.NewAPIDefinition(IntegrationName)
	if err != nil {
		return nil, fmt.Errorf("cannot create API definition for HTTP API server, err: %s", err)
	}

	return &Server{
		logger:     log.WithComponent(componentName),
		reporter:   r,
		definition: d,
		emitter:    em,
		readyCh:    make(chan struct{}),
	}, nil
}

// Serve serves status API requests.
// Nice2Have: context cancellation.
func (s *Server) Serve(ctx context.Context) {
	if s.Status.enabled {
		go func() {
			s.logger.WithFields(logrus.Fields{
				"address": s.Status.address,
			}).Debug("Status API starting listening.")

			router := httprouter.New()
			// read only API
			router.GET(statusAPIPathReady, s.handleReady)
			router.GET(statusEntityAPIPath, s.handleEntity)
			router.GET(statusAPIPath, s.handle(false))
			router.GET(statusOnlyErrorsAPIPath, s.handle(true))
			// local only API
			err := http.ListenAndServe(s.Status.address, router)
			if err != nil {
				s.logger.WithError(err).Error("unable to start Status-API")
				return
			}
			s.logger.Debug("Status API stopped.")
		}()
	}

	if s.Ingest.enabled {
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
		if !ingestReady && s.Ingest.enabled {
			scheme := "http://"
			if s.Ingest.tls.enabled {
				scheme = "https://"
				c.Transport = &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				}
			}

			if s.Ingest.tls.validateClient {
				// If client validation is enabled, we cannot probe the /ready path wihtout a valid certificate, which
				// might not be available to us. For this reason we must lie and tell it is ready without probing.
				ingestReady = true
			} else {
				ingestReady = s.isGetSuccessful(c, scheme+s.Ingest.address+ingestAPIPathReady)
			}
		}
		if !statusReady && s.Status.enabled {
			scheme := "http://"
			statusReady = s.isGetSuccessful(c, scheme+s.Status.address+statusAPIPathReady)
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
		"address": s.Ingest.address,
	}).Debug("Ingest API starting listening.")

	router := httprouter.New()
	router.GET(ingestAPIPathReady, s.handleReady)
	router.POST(ingestAPIPath, s.handleIngest)

	server := &http.Server{
		Handler: router,
		Addr:    s.Ingest.address,
	}

	if s.Ingest.tls.enabled {
		if s.Ingest.tls.validateClient {
			err := addMTLS(server, s.Ingest.tls.caPath)
			if err != nil {
				return fmt.Errorf("creating mTLS server: %w", err)
			}
		}

		s.logger.Debug("starting tls server")
		err := server.ListenAndServeTLS(s.Ingest.tls.certPath, s.Ingest.tls.keyPath)
		if err != nil {
			return fmt.Errorf("starting tls server: %w", err)
		}
	}

	err := server.ListenAndServe()
	if err != nil {
		return fmt.Errorf("starting Ingest server: %w", err)
	}

	return ErrServerStopped
}

func (s *Server) allReadyOrDisabled(ingestReady, statusReady bool) bool {
	if s.Ingest.enabled && !ingestReady {
		return false
	}
	if s.Status.enabled && !statusReady {
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
		s.logger.WithError(err).Warnf("httpapi readiness probe failed")
		return false
	}

	return resp.StatusCode == http.StatusOK
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
