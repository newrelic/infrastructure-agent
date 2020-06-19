// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"io/ioutil"
	"net/http"

	. "gopkg.in/check.v1"
)

type TestSuite struct{}

var _ = Suite(&TestSuite{})

func (s *TestSuite) TestMockTransport(c *C) {
	mock := NewMockTransport()
	client := &http.Client{
		Transport: mock,
	}
	mock.Append(200, []byte(`foo`))
	s.Get(client, `foo`, c)
	// try again
	s.Get(client, `foo`, c)
	// should get the empty value now
	mock.WhenEmpty(200, []byte(`bar`))
	s.Get(client, `bar`, c)
	s.Get(client, `bar`, c)
	// and not anymore
	mock.Append(200, []byte(`foo`))
	s.Get(client, `foo`, c)
}

func (s *TestSuite) Get(client *http.Client, expected string, c *C) {
	resp, err := client.Get("http://jsonip.com")
	c.Assert(err, IsNil)
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	c.Assert(err, IsNil)
	c.Assert(body, NotNil)
	c.Assert(string(body), Equals, expected)
}
