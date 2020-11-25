// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

func Test_configureLogRedirection(t *testing.T) {
	// Given a new MemLogger with data
	l := log.NewMemLogger(ioutil.Discard)
	_, err := l.Write([]byte("example logs here"))
	require.NoError(t, err)

	// And a log file
	logFile, err := ioutil.TempFile("", "newLogs.txt")
	require.NoError(t, err)

	// When log redirection is configured to log file
	assert.True(t, configureLogRedirection(&config.Config{
		LogFile: logFile.Name(),
	}, l))

	// Then data previously stored in MemLogger gets written into log file
	dat, err := ioutil.ReadFile(logFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "example logs here", string(dat))
}
