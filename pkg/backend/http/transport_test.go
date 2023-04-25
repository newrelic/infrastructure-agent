// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"bytes"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestRequestDecorator_AddHeaders(t *testing.T) {
	cfg := &config.Config{
		Http: config.HttpConfig{
			Headers: map[string]string{
				"Content-Encoding": "gzip2",
			},
		},
	}
	mock := NewRequestInterceptorMock()

	rdt := NewRequestDecoratorTransport(cfg, mock)

	req, err := http.NewRequest(http.MethodPost, "test", bytes.NewReader([]byte{}))
	assert.NoError(t, err)

	_, err = rdt.RoundTrip(req)
	assert.NoError(t, err)

	req = <-mock.req

	assert.EqualValues(t, req.Header, map[string][]string{
		"Content-Encoding": {"gzip2"},
	})
}
