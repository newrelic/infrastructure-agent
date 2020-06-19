// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"compress/gzip"
	"flag"
	"runtime"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	metrics_sender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/test/infra"
	"github.com/newrelic/infrastructure-agent/test/proxy/minagent"
	"github.com/sirupsen/logrus"
)

// minimalist agent. It loads the configuration from the environment and the file passed by the -config flag.
// It just submits `FakeSample` instances to the collector.
func main() {
	malog := logrus.WithField("component", "minimal-standalone-agent")

	logrus.Info("Runing minimalistic test agent...")
	runtime.GOMAXPROCS(1)

	configFile := flag.String("config", minagent.DefaultConfig, "configuration file")
	flag.Parse()

	cfg, err := config.LoadConfigWithVerbose(*configFile, 0)
	if err != nil {
		malog.WithError(err).Fatal("can't load configuration file")
	}

	if cfg.CABundleFile == "" && cfg.CABundleDir == "" {
		cfg.CABundleDir = "/cabundle"
	}
	cfg.PayloadCompressionLevel = gzip.NoCompression

	a := infra.NewAgentFromConfig(cfg)
	sender := metrics_sender.NewSender(a.Context)
	sender.RegisterSampler(&minagent.FakeSampler{})
	a.RegisterMetricsSender(sender)

	if err := a.Run(); err != nil {
		malog.WithError(err).Error("while starting agent")
	}
}
