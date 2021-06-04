// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
)

// NoopMeasure no-op Measure function type.
var NoopMeasure = func(_ MetricType, _ MetricName, _ int64) {}

// NewNoop creates a new no-op Instrumenter.
func NewNoop() (exporter Instrumenter) {
	return &noop{}
}

type noop struct {
}

func (n noop) Measure(_ MetricType, _ MetricName, _ int64) {
}

func (n noop) GetHandler() http.Handler {
	logrus.Warn("This is not supported on OS")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Write([]byte("# WARN This is not supported on OS"))
		_, _ = ioutil.ReadAll(r.Body)
		w.WriteHeader(200)
	})
}

func (n noop) GetHttpTransport(base http.RoundTripper) http.RoundTripper {
	return base
}
