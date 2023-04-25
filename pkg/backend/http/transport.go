// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"net/http"
)

// requestDecorator is able to decorate the http requests with extra info. E.g. Adding proxy headers.
type requestDecorator struct {
	rt           http.RoundTripper
	configurator config.Provider
}

// NewRequestDecoratorTransport comes with ability to decorate http request objects.
func NewRequestDecoratorTransport(configurator config.Provider, transport http.RoundTripper) http.RoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &requestDecorator{
		rt:           transport,
		configurator: configurator,
	}
}

func (t *requestDecorator) RoundTrip(req *http.Request) (*http.Response, error) {
	cfg := t.configurator.Provide()

	for key, val := range cfg.Http.Headers {
		req.Header.Add(key, val)
	}

	return t.rt.RoundTrip(req)
}
