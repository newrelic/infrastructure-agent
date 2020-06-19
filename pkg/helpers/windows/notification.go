// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package windows

import (
	"bufio"
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
)

// PipeName is the name of the NamedPipe used for notifications
const PipeName = `\\.\pipe\newrelic-infra`

// PostNotificationMessage sends the message argument to the NamedPipe
func PostNotificationMessage(pipeName string, message ipc.Message) (err error) {
	conn, err := winio.DialPipe(pipeName, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = writeMessage(fmt.Sprint(message)+"\n", conn)
	return err
}

func getConnection(pipeName string) (conn net.Conn, err error) {
	conn, err = winio.DialPipe(pipeName, nil)
	return conn, err
}

func writeMessage(message string, conn net.Conn) (err error) {
	w := bufio.NewWriter(conn)
	defer w.Flush()
	_, err = w.WriteString(message)
	return
}

// NewNotificationPipeListener creates and returns a NamedPipe listener
// that can be used to accept new connection requests
// the client is responsible for closing the listener
func NewNotificationPipeListener(pipeName string) (l net.Listener, err error) {
	l, err = winio.ListenPipe(pipeName, nil)
	return
}
