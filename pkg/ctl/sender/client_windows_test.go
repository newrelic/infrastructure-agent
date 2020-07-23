// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package sender

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/windows"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/stretchr/testify/assert"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestNewClient(t *testing.T) {
	ready := make(chan int)

	// setup "server"
	pipeName := getPipeName(t)
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

func getPipeName(t *testing.T) string {
	return windows.GetPipeName(t.Name() + randSeq(12))
}

func TestEnableVerboseMissingPipe(t *testing.T) {
	pipeName := getPipeName(t)
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
