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
	"io/ioutil"
	"net"
	"net/http"
	"sync"
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
	statusHealthAPIPath        = "/v1/status/health"
	ingestAPIPath              = "/v1/data"
	ingestAPIPathReady         = "/v1/data/ready"
	readinessProbeRetryBackoff = 100 * time.Millisecond
)

const readinessProbeTimeout = time.Second * 5

var ErrURLUnreachable = errors.New("cannot reach url")

type responseError struct {
	Error string `json:"error"`
}

// Server runtime for status API server.
type Server struct {
	Ingest        ComponentConfig
	Status        ComponentConfig
	reporter      status.Reporter
	logger        log.Entry
	definition    integration.Definition
	emitter       emitter.Emitter
	statusReadyCh chan struct{}
	ingestReadyCh chan struct{}
	timeout       time.Duration
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
		logger:        log.WithComponent(componentName),
		reporter:      r,
		definition:    d,
		emitter:       em,
		ingestReadyCh: make(chan struct{}),
		statusReadyCh: make(chan struct{}),
		timeout:       readinessProbeTimeout,
	}, nil
}

// Serve serves status API requests and ingest.
// Nice2Have: context cancellation.
func (s *Server) Serve(ctx context.Context) {
	if !s.Status.enabled && !s.Ingest.enabled {
		return
	}

	var serversWg sync.WaitGroup
	var statusErr, ingestErr error

	if s.Status.enabled {
		serversWg.Add(1)
		go func() {
			statusErr = s.serveStatus(ctx)
			if statusErr != nil {
				s.logger.WithError(statusErr).Error("error serving agent status")
			}
			close(s.statusReadyCh)
			serversWg.Done()
		}()
	} else {
		close(s.statusReadyCh)
	}

	if s.Ingest.enabled {
		serversWg.Add(1)
		go func() {
			ingestErr = s.serveIngest(ctx)
			if ingestErr != nil {
				s.logger.WithError(ingestErr).Error("error serving agent ingest")
			}
			close(s.ingestReadyCh)
			serversWg.Done()
		}()
	} else {
		close(s.ingestReadyCh)
	}

	serversWg.Wait()

	if statusErr != nil && ingestErr != nil {
		return
	}

	<-ctx.Done()
}

// serveStatus serves status API requests.
func (s *Server) serveStatus(_ context.Context) error {
	statusServerErr := make(chan error, 1)

	go func() {
		defer close(statusServerErr)
		s.logger.WithFields(logrus.Fields{
			"address": s.Status.address,
		}).Debug("Status API starting listening.")

		router := httprouter.New()
		// read only API
		router.GET(statusAPIPathReady, s.handleReady)
		router.GET(statusEntityAPIPath, s.handleEntity)
		router.GET(statusAPIPath, s.handle(false))
		router.GET(statusOnlyErrorsAPIPath, s.handle(true))
		router.GET(statusHealthAPIPath, s.handleHealth)
		// local only API
		err := http.ListenAndServe(s.Status.address, router)
		statusServerErr <- err

		if err != nil {
			s.logger.WithError(err).Error("unable to start Status-API")

			return
		}

		s.logger.Debug("Status API stopped.")
	}()

	return s.waitUntilReadyOrError(s.Status.address, statusAPIPathReady, s.Status.tls.enabled, s.Status.tls.validateClient, statusServerErr)
}

// serveIngest creates and starts an HTTP server handling ingestAPIPathReady and ingestAPIPath using Config.Ingest
func (s *Server) serveIngest(_ context.Context) error {
	serverErr := make(chan error, 1)

	go func() {
		defer close(serverErr)
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
					serverErr <- fmt.Errorf("creating mTLS server: %w", err)

					return
				}
			}

			s.logger.Debug("starting tls server")
			err := server.ListenAndServeTLS(s.Ingest.tls.certPath, s.Ingest.tls.keyPath)
			if err != nil {
				serverErr <- fmt.Errorf("starting tls server: %w", err)

				return
			}
		}

		err := server.ListenAndServe()
		if err != nil {
			s.logger.WithError(err).Error("Ingest server error")
		}
		serverErr <- err
	}()

	return s.waitUntilReadyOrError(s.Ingest.address, ingestAPIPathReady, s.Ingest.tls.enabled, s.Ingest.tls.validateClient, serverErr)
}

// waitUntilReadyOrError makes http request to address if server didn't return an error.
// It has a timeout to prevent staying in an infinite loop
func (s *Server) waitUntilReadyOrError(address string, path string, tlsEnabled bool, validateTLSClient bool, serverErrCh <-chan error) error {
	var err error
	client := http.Client{}
	scheme := "http"

	if tlsEnabled {
		scheme = "https"
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		}
	}

	url := fmt.Sprintf("%s://%s%s", scheme, address, path)
	timer := time.NewTimer(s.timeout)

	for {
		// If client validation is enabled, we cannot probe the path wihtout a valid certificate, which
		// might not be available to us. For this reason we must lie and tell it is ready without probing.
		if (tlsEnabled && validateTLSClient) || s.isGetSuccessful(client, url) {
			break
		}
		// if the server is not running and returned an error we stop trying and return the error
		select {
		case err = <-serverErrCh:
			if err != nil {
				return err
			}
		case <-timer.C:
			err = fmt.Errorf("error reading url:%s %w", url, ErrURLUnreachable)

			return err
		default:
		}
		time.Sleep(readinessProbeRetryBackoff)
	}

	return err
}

// isGetSuccessful makes a http request to URL and returns true statusCode == 200.
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

// waitUntilReady blocks the call until server is ready to accept connections.
// currently only used in tests
func (s *Server) waitUntilReady() {
	<-s.ingestReadyCh
	<-s.statusReadyCh
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

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleHealth(writer http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	health := s.reporter.ReportHealth()

	body, err := json.Marshal(health)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		s.logger.WithError(err).Warn("couldn't encode Status report")

		return
	}

	if !health.Healthy {
		writer.WriteHeader(http.StatusInternalServerError)
	}

	_, err = writer.Write(body)
	if err != nil {
		s.logger.Warn("cannot write entity response, error: " + err.Error())
		writer.WriteHeader(http.StatusInternalServerError)

		return
	}
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
