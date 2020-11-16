package socketapi

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const IntegrationName = "socket-api"

// Server runtime for socket API server.
type Server struct {
	port    int
	logger  log.Entry
	emitter emitter.Emitter
	il      integration.InstancesLookup
}

type sockHandleF func() error

// NewServer creates a new socket API server.
func NewServer(emitter emitter.Emitter, il integration.InstancesLookup) *Server {
	logger := log.WithComponent("Server")
	return &Server{
		port:    7070,
		logger:  logger,
		il:      il,
		emitter: emitter,
	}
}

// Serve serves socket API requests.
func (s *Server) Serve(ctx context.Context) {
	def, err := integration.NewDefinition(config.ConfigEntry{
		InstanceName: IntegrationName,
	}, s.il, nil, nil)
	if err != nil {
		s.logger.WithError(err).Error("cannot create integration definition")
		return
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		s.logger.WithField("port", s.port).WithError(err).Error("trying to listen")
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:

		}

		// this is a PoC! just handling 1 connection fir this purpose is enough
		conn, err := listener.Accept()
		if err != nil {
			s.logger.WithField("port", s.port).WithError(err).Error("cannot accept connection")
		}
		defer func() {
			if err = conn.Close(); err != nil {
				s.logger.WithError(err).Error("cannot close connection")
			}
		}()

		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err == io.EOF {
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
}
