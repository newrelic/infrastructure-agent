// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package infra

import (
	"compress/gzip"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/ctl"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
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
	return NewAgentWithConnectClient(SuccessConnectClient, dataClient, configurator...)
}

func NewAgentFromConfig(cfg *config.Config) *agent.Agent {
	transport := backendhttp.BuildTransport(cfg, backendhttp.ClientTimeout)
	dataClient := backendhttp.GetHttpClient(backendhttp.ClientTimeout, transport)
	return NewAgentWithConnectClientAndConfig(SuccessConnectClient, dataClient.Do, cfg)
}

// NewAgentWithConnectClient create a new agent for testing with a provided connect-client.
func NewAgentWithConnectClient(connectClient, dataClient backendhttp.Client, configurator ...func(*config.Config)) *agent.Agent {
	return NewAgentWithConnectClientAndConfig(connectClient, dataClient, &config.Config{
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
	}, configurator...)
}

func NewAgentWithConnectClientAndConfig(connectClient, dataClient backendhttp.Client, cfg *config.Config, configurator ...func(*config.Config)) *agent.Agent {

	for _, c := range configurator {
		c(cfg)
	}

	ctx := agent.NewContext(cfg, "1.2.3", testhelpers.NewFakeHostnameResolver("foobar", "foo", nil), nil, matcher)

	if cfg.AgentDir == "" {
		var err error
		cfg.AgentDir, err = ioutil.TempDir("", "prefix")
		if err != nil {
			panic(err)
		}
	}
	dataDir := filepath.Join(cfg.AgentDir, "data")
	st := delta.NewStore(dataDir, "default", cfg.MaxInventorySize)

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)

	lookups := agent.NewIdLookup(hostname.CreateResolver(cfg.OverrideHostname, cfg.OverrideHostnameShort, cfg.DnsHostnameResolution), cloudDetector, cfg.DisplayName)

	fingerprintHarvester, err := fingerprint.NewHarvestor(cfg, testhelpers.NullHostnameResolver, cloudDetector)

	if err != nil {
		panic(err)
	}

	connectC, err := identityapi.NewIdentityConnectClient("url", "license", "user-agent", gzip.BestCompression, true, connectClient)
	if err != nil {
		panic(err)
	}

	connectSrv := agent.NewIdentityConnectService(connectC, fingerprintHarvester)

	registerC, err := identityapi.NewIdentityRegisterClient("url", "license", "user-agent", gzip.BestCompression, connectClient)
	if err != nil {
		panic(err)
	}

	provideIDs := agent.NewProvideIDs(registerC, state.NewRegisterSM())
	transport := backendhttp.BuildTransport(cfg, backendhttp.ClientTimeout)
	a, err := agent.New(cfg, ctx, "user-agent", lookups, st, delta.NewLastSubmissionInMemory(), connectSrv, provideIDs, dataClient, transport, cloudDetector, fingerprintHarvester, ctl.NewNotificationHandlerWithCancellation(nil))
	if err != nil {
		panic(err)
	}

	return a
}
