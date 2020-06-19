// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package log

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var logText = `time="2019-07-08T09:03:49Z" level=debug msg="Using collector URL: https://staging-infra-api.newrelic.com"
time="2019-07-08T09:03:49Z" level=debug msg="Using InventoryIngestEndpoint: /inventory"
time="2019-07-08T09:03:49Z" level=debug msg="Using MetricsIngestEndpoint: /metrics"
time="2019-07-08T09:03:49Z" level=debug msg="Using IdentityIngestEndpoint: /identity/v1"
time="2019-07-08T09:03:49Z" level=debug msg="Using default output directory: /var/db/newrelic-infra"`

func TestMemLoggerToMem(t *testing.T) {
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, r.Close())
		assert.NoError(t, w.Close())
	}()

	log := NewMemLogger(w)

	_, err = log.Write([]byte(logText))
	assert.NoError(t, err)

	stdOut := make(chan string)
	go func() {
		buf := make([]byte, len(logText))
		_, err := r.Read(buf)
		assert.NoError(t, err)
		stdOut <- string(buf)
	}()

	out := <-stdOut
	assert.EqualValues(t, logText, out)
}

func TestMemLoggerToFile(t *testing.T) {
	log := NewMemLogger(ioutil.Discard)

	_, err := log.Write([]byte(logText))
	assert.NoError(t, err)

	// test if memLogger is able to write the buffer to a file
	r, w, err := os.Pipe()
	assert.NoError(t, err)

	_, err = log.WriteBuffer(w)
	assert.NoError(t, err)

	stdOut := make(chan string)
	go func() {
		buf := make([]byte, len(logText))
		_, err := r.Read(buf)
		assert.NoError(t, err)
		stdOut <- string(buf)
	}()

	out := <-stdOut
	assert.EqualValues(t, logText, out)
}
