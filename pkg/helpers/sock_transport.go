// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"net"
	"net/http"
)

type PersistentSocketTransport struct {
	c *http.Client
}

func (pst *PersistentSocketTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return pst.c.Do(req) //nolint:wrapcheck
}

func NewSocketTransport(path string) (*PersistentSocketTransport, error) {
	_, err := socketDial("unix", path)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{ //nolint:exhaustruct
		Transport: &http.Transport{ //nolint:exhaustruct
			Dial: socketDial,
		},
	}

	return &PersistentSocketTransport{httpClient}, nil
}

func socketDial(_, addr string) (net.Conn, error) {
	return net.Dial("unix", addr) //nolint:wrapcheck
}
