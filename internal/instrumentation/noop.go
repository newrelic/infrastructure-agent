// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"io/ioutil"
	"net/http"
)

func NewNoopServer() (exporter Exporter) {
	return &noop{}
}

type noop struct {
}

func (n noop) IncrementSomething(_ int64) {

}

func (n noop) GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_, _ = ioutil.ReadAll(r.Body)
		w.WriteHeader(200)
	})
}

func (n noop) GetHttpTransport(base http.RoundTripper) http.RoundTripper {
	return base
}
