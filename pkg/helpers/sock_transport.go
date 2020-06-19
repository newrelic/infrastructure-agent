// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"net"
	"net/http"
	"net/http/httputil"
)

type PersistentSocketTransport struct {
	c *httputil.ClientConn
}

func (self *PersistentSocketTransport) RoundTrip(req *http.Request) (r *http.Response, err error) {
	r, err = self.c.Do(req)
	return
}

func NewSocketTransport(path string) (*PersistentSocketTransport, error) {
	var (
		c   net.Conn
		err error
	)
	if c, err = net.Dial("unix", path); err != nil {
		return nil, err
	}
	clientConn := httputil.NewClientConn(c, nil)
	return &PersistentSocketTransport{clientConn}, nil
}
