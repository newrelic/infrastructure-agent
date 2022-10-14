// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"time"

	kardianosService "github.com/kardianos/service"
	"github.com/newrelic/infrastructure-agent/internal/agent/service"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/opamp"
)

type supervisor struct {
	svc             kardianosService.Service
	opampClient     *opamp.Client
	ctx             context.Context
	configChangedCh <-chan struct{}
}

var slog = log.WithComponent("supervisor")

func newSupervisor(ctx context.Context, arg ...string) (*supervisor, error) {
	svc, err := service.New(arg...)
	if err != nil {
		return nil, err
	}

	configChangedCh := make(chan struct{}, 1)
	opampClient, err := opamp.NewClient(
		ctx,
		configChangedCh,
		log.WithField("client", "OpAMP Client"),
		buildVersion,
	)

	return &supervisor{
		svc:             svc,
		opampClient:     opampClient,
		ctx:             ctx,
		configChangedCh: configChangedCh,
	}, nil
}

func (s *supervisor) run() error {
	time.AfterFunc(time.Second*20, func() {
		err := s.opampClient.Start()
		if err != nil {
			slog.WithError(err).Error("cannot start opamp client")
		}
	})

	go func() {
		slog.Infof("waiting for config changed messages")
		<-s.configChangedCh
		slog.Infof("config change received")
		err := s.svc.Restart()
		if err != nil {
			slog.WithError(err).Error("canot restart service")
		}
	}()

	if err := s.svc.Run(); err != nil {
		return err
	}

	return nil
}
