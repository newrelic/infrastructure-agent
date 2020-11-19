// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockTransport(t *testing.T) {
	mock := NewMockTransport()
	client := &http.Client{
		Transport: mock,
	}
	mock.Append(200, []byte(`foo`))
	assertGet(t, client, `foo`)
	// try again
	assertGet(t, client, `foo`)
	// should get the empty value now
	mock.WhenEmpty(200, []byte(`bar`))
	assertGet(t, client, `bar`)
	assertGet(t, client, `bar`)
	// and not anymore
	mock.Append(200, []byte(`foo`))
	assertGet(t, client, `foo`)
}

func assertGet(t *testing.T, client *http.Client, expected string) {
	resp, err := client.Get("http://unit.test")
	assert.NoError(t, err)
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEmpty(t, body)
	assert.Equal(t, expected, string(body))
}
