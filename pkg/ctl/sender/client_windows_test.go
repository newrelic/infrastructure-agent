// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package sender

import (
	"bufio"
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/windows"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/stretchr/testify/assert"

	"testing"
)

const pipeName = windows.PipeName + "-test"

func TestNewClient(t *testing.T) {
	ready := make(chan int)

	// setup "server"
	l, err := winio.ListenPipe(pipeName, nil)
	assert.NoError(t, err)
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		assert.NoError(t, err)
		defer conn.Close()

		s, err := handleRequest(conn)
		assert.NoError(t, err)
		assert.NotEmpty(t, s)
		ready <- 1
	}()

	c, err := NewClientWithName(0, pipeName)

	assert.NoError(t, err)

	err = c.Notify(nil, ipc.EnableVerboseLogging)
	assert.NoError(t, err)
	<-ready
}

func TestEnableVerboseMissingPipe(t *testing.T) {
	c, err := NewClientWithName(0, pipeName)

	err = c.Notify(nil, ipc.EnableVerboseLogging)
	assert.EqualError(t, err, fmt.Sprintf("open %s: The system cannot find the file specified.", pipeName))
}

func handleRequest(conn net.Conn) (s string, err error) {
	reader := bufio.NewReader(conn)
	// read everything until newline (including)
	s, err = reader.ReadString('\n')
	return s, err
}
