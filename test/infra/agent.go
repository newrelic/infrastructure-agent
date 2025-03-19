// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package infra

import (
	"compress/gzip"
	"context"

	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	infra "github.com/newrelic/infrastructure-agent/test/infra/http" //nolint:depguard

	"github.com/newrelic/infrastructure-agent/pkg/ctl"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostid" //nolint:depguard

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

// Matcher function that will allow process to be sent
var matcher = func(interface{}) bool { return true }

// Create a new agent for testing.
func NewAgent(dataClient backendhttp.Client, configurator ...func(*config.Config)) *agent.Agent {
	return NewAgentWithConnectClient(NewSuccessConnectHttpClient(), dataClient, configurator...)
}

func NewAgentFromConfig(cfg *config.Config) *agent.Agent {
	transport := backendhttp.BuildTransport(cfg, backendhttp.ClientTimeout)
	transport = backendhttp.NewRequestDecoratorTransport(cfg, transport)
	dataClient := backendhttp.GetHttpClient(backendhttp.ClientTimeout, transport)
	return NewAgentWithConnectClientAndConfig(NewSuccessConnectHttpClient(), dataClient.Do, cfg)
}

// NewAgentWithConnectClient create a new agent for testing with a provided connect-client.
func NewAgentWithConnectClient(connectClient *http.Client, dataClient backendhttp.Client, configurator ...func(*config.Config)) *agent.Agent {
	cfg := &config.Config{
		IgnoredInventoryPaths:    []string{"test/plugin/yum"},
		MaxInventorySize:         config.DefaultMaxMetricsBatchSizeBytes,
		DisplayName:              "display-name",
		FirstReapInterval:        time.Millisecond,
		ReapInterval:             time.Millisecond,
		SendInterval:             time.Millisecond,
		FingerprintUpdateFreqSec: 60,
		StartupConnectionRetries: 3,
		StartupConnectionTimeout: "5s",
		OfflineTimeToReset:       config.DefaultOfflineTimeToReset,
		Http:                     config.NewHttpConfig(),
	}

	return NewAgentWithConnectClientAndConfig(connectClient, dataClient, cfg, configurator...)
}

func NewAgentWithConnectClientAndConfig(connectClient *http.Client, dataClient backendhttp.Client, cfg *config.Config, configurator ...func(*config.Config)) *agent.Agent {
	for _, c := range configurator {
		c(cfg)
	}

	if cfg.AgentDir == "" {
		var err error
		cfg.AgentDir, err = ioutil.TempDir("", "prefix")
		if err != nil {
			panic(err)
		}
	}
	dataDir := filepath.Join(cfg.AgentDir, "data")
	st := delta.NewStore(dataDir, "default", cfg.MaxInventorySize, cfg.InventoryArchiveEnabled)

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)

	lookups := agent.NewIdLookup(hostname.CreateResolver(cfg.OverrideHostname, cfg.OverrideHostnameShort, cfg.DnsHostnameResolution), cloudDetector, cfg.DisplayName)

	ctx := agent.NewContext(cfg, "1.2.3", testhelpers.NewFakeHostnameResolver("foobar", "foo", nil), lookups, matcher, matcher)

	fingerprintHarvester, err := fingerprint.NewHarvestor(cfg, testhelpers.NullHostnameResolver, cloudDetector)
	if err != nil {
		panic(err)
	}

	connectClient.Transport = backendhttp.NewRequestDecoratorTransport(cfg, connectClient.Transport)

	connectC, err := identityapi.NewIdentityConnectClient("url", "license", "user-agent", gzip.BestCompression, true, connectClient.Do)
	if err != nil {
		panic(err)
	}

	connectMetadataHarvester := identityapi.NewMetadataHarvesterDefault(hostid.NewProviderEnv())

	connectSrv := agent.NewIdentityConnectService(connectC, fingerprintHarvester, connectMetadataHarvester)

	registerC, err := identityapi.NewRegisterClient(
		"url",
		"license", ""+
			"user-agent",
		gzip.BestCompression,
		connectClient,
	)
	if err != nil {
		panic(err)
	}

	provideIDs := agent.NewProvideIDs(registerC, state.NewRegisterSM())
	transport := backendhttp.BuildTransport(cfg, backendhttp.ClientTimeout)
	transport = backendhttp.NewRequestDecoratorTransport(cfg, transport)
	dataClient = backendhttp.NewRequestDecoratorTransport(cfg, infra.ToRoundTripper(dataClient)).RoundTrip
	ffRetriever := feature_flags.NewManager(map[string]bool{})
	agent, err := agent.New(cfg, ctx, "user-agent", lookups, st, connectSrv, provideIDs, dataClient, transport, cloudDetector, fingerprintHarvester, ctl.NewNotificationHandlerWithCancellation(context.TODO()), ffRetriever)
	if err != nil {
		panic(err)
	}

	agent.Init()

	return agent
}
