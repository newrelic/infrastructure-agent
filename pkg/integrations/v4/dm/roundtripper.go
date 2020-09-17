// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"net/http"

	"github.com/newrelic/infrastructure-agent/internal/agent/id"
)

const (
	agentEntityHeader    = "X-NRI-Agent-Entity-Id"
	licenseKeyHeader     = "X-License-Key"
	apiKeyHeaderToRemove = "Api-Key"
)

type roundTripper struct {
	rt         http.RoundTripper
	licenseKey string
	idProvide  id.Provide
}

// newTransport comes with usual agent proxy setup and submission timeouts.
func newTransport(agentTransport http.RoundTripper, licenseKey string, idProvide id.Provide) *roundTripper {
	if agentTransport == nil {
		agentTransport = http.DefaultTransport
	}

	return &roundTripper{
		rt:         agentTransport,
		licenseKey: licenseKey,
		idProvide:  idProvide,
	}
}

func (t roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Use license key header rather than API key
	req.Header.Del(apiKeyHeaderToRemove)
	req.Header.Add(licenseKeyHeader, t.licenseKey)
	req.Header.Add(agentEntityHeader, t.idProvide().ID.String())
	return t.rt.RoundTrip(req)
}
