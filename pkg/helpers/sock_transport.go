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
	_, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}

	return &PersistentSocketTransport{http.DefaultClient}, nil
}
