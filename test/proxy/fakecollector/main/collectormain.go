// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/tls"
	"net/http"

	"github.com/newrelic/infrastructure-agent/test/proxy/fakecollector"
	"github.com/newrelic/infrastructure-agent/test/proxy/testsetup"
	"github.com/sirupsen/logrus"
)

// The fake collector is a simple https service that ingests the metrics from the agent, and enables extra
// endpoints to be controlled and monitored from the tests.
// It stores in a queue all the events that it receives from the agent.
func main() {
	logrus.Info("Runing fake collector...")

	collector := fakecollector.NewService()

	mux := http.NewServeMux()
	mux.HandleFunc("/identity/v1/connect", collector.Connect)            // fake connect service
	mux.HandleFunc("/infra/v2/metrics/events/bulk", collector.NewSample) // fake ingest service
	mux.HandleFunc("/metrics/events/bulk", collector.NewSample)          // fake ingest service for old endpoint.
	mux.HandleFunc("/notifyproxy", collector.NotifyProxy)                // the proxy uses this endpoint to notify it is sending data
	mux.HandleFunc("/nextevent", collector.DequeueSample)                // returns the next received event in the queue
	mux.HandleFunc("/lastproxy", collector.LastProxy)                    // returns the proxies that have been used
	mux.HandleFunc("/cleanup", collector.ClearQueue)                     // cleans up the events queue and the last proxy name

	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
	srv := &http.Server{
		Addr:         ":4444",
		Handler:      mux,
		TLSConfig:    cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}
	srv.ListenAndServeTLS(testsetup.CollectorCertFile, testsetup.CollectorKeyFile)
}
