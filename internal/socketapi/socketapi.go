// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package socketapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const IntegrationName = "socket-api"

// Server runtime for socket API server.
type Server struct {
	host    string
	port    int
	logger  log.Entry
	emitter emitter.Emitter
	readyCh chan struct{}
}

// NewServer creates a new socket API server.
func NewServer(emitter emitter.Emitter, host string, port int) *Server {
	logger := log.WithComponent("Server")
	return &Server{
		host:    host,
		port:    port,
		logger:  logger,
		emitter: emitter,
		readyCh: make(chan struct{}),
	}
}

// Serve serves socket API requests.
func (s *Server) Serve(ctx context.Context) {
	def, err := integration.NewAPIDefinition(IntegrationName)
	if err != nil {
		s.logger.WithError(err).Error("cannot create integration definition")
		return
	}

	//nolint:exhaustruct
	lc := net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		s.logger.WithField("port", s.port).WithError(err).Error("trying to listen")
		return
	}
	defer listener.Close()

	close(s.readyCh)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			s.logger.WithField("port", s.port).WithError(err).Error("cannot accept connection")

			continue
		}

		s.handleConn(def, conn)
	}
}

// WaitUntilReady blocks the call until server is ready to accept connections.
func (s *Server) WaitUntilReady() {
	_, _ = <-s.readyCh
}

func (s *Server) handleConn(def integration.Definition, conn net.Conn) {
	defer func() {
		err := conn.Close()
		if err != nil {
			s.logger.WithError(err).Error("cannot close connection")
		}
	}()

	r := bufio.NewReader(conn)

	for {
		line, err := r.ReadString('\n')
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			s.logger.WithError(err).Warn("cannot read connection")

			break
		}

		line = strings.TrimSuffix(line, "\n")

		err = s.emitter.Emit(def, nil, nil, []byte(line))
		if err != nil {
			s.logger.WithError(err).Error("cannot emit payload")
		}
	}
}
