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

func TestLogRedirection(t *testing.T) {
	logFile, err := ioutil.TempFile("", "newLogs.txt")
	require.NoError(t, err)
	logText := "example logs here"
	_, _ = logFile.WriteString(logText)
	cfg := &config.Config{
		LogFile: logFile.Name(),
	}
	assert.True(t, configureLogRedirection(cfg, &log.MemLogger{}))
	dat, err := ioutil.ReadFile(logFile.Name())
	require.NoError(t, err)
	assert.Equal(t, logText, string(dat))
}
