// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"os/user"

	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

const (
	// Plain text log format.
	LogFormatText = "text"
	// JSON log format.
	LogFormatJSON = "json"

	// Non configurable stuff
	defaultIdentityURLEu          = "https://identity-api.eu.newrelic.com"
	defaultIdentityStagingURLEu   = "https://staging-identity-api.eu.newrelic.com"
	defaultCmdChannelURLEu        = "https://infrastructure-command-api.eu.newrelic.com"
	defaultCmdChannelStagingURLEu = "https://staging-infrastructure-command-api.eu.newrelic.com"
	defaultCmdChannelURL          = "https://infrastructure-command-api.newrelic.com"
	defaultCmdChannelStagingURL   = "https://staging-infrastructure-command-api.newrelic.com"
	defaultIdentityURL            = "https://identity-api.newrelic.com"
	defaultIdentityStagingURL     = "https://staging-identity-api.newrelic.com"
	baseCollectorURL              = "https://%sinfra-api.%snewrelic.com"
	baseDimensionalMetricURL      = "https://%smetric-api.%snewrelic.com"
	defaultSecureFederalURL       = "https://gov-infra-api.newrelic.com"
)

// Default configurable values
var (
	// public
	DefaultContainerCacheMetadataLimit = 60
	DefaultDockerApiVersion            = "1.24" // minimum supported API by Docker 18.09.0
	DefaultHeartBeatFrequencySecs      = 60
	DefaultDMPeriodSecs                = 5           // default telemetry SDK value
	DefaultMaxMetricsBatchSizeBytes    = 1000 * 1000 // Size limit from Vortex collector service (1MB)
	DefaultMaxMetricBatchEntitiesCount = 300         // Amount limit from Vortex collector service header (8k ~ 300 entities)
	DefaultMaxMetricBatchEntitiesQueue = 1000        // Limit the amount of queued entities to be processed by Vortex collector service
	DefaultMetricsNFSSampleRate        = 20
	DefaultOfflineTimeToReset          = "24h"
	DefaultStorageSamplerRateSecs      = 20
	DefaultStripCommandLine            = true
	DefaultSmartVerboseModeEntryLimit  = 1000
	DefaultIntegrationsDir             = "newrelic-integrations"
	DefaultInventoryQueue              = 0

	// private
	defaultAppDataDir                    = ""
	defaultCmdChannelEndpoint            = "/agent_commands/v1/commands"
	defaultCmdChannelIntervalSec         = 60
	defaultCompactEnabled                = true
	defaultCompactThreshold              = 20 * 1024 * 1024 // (in bytes) compact repo when it hits 20MB
	defaultIgnoreReclaimable             = false
	defaultDebugLogSec                   = 600
	defaultDisableInventorySplit         = false
	defaultDisableWinSharedWMI           = false
	defaultDisableZeroRSSFilter          = false
	defaultDnsHostnameResolution         = true
	defaultFilesConfigOn                 = false
	defaultMaxProcs                      = 1
	defaultHTTPServerHost                = "localhost"
	defaultHTTPServerPort                = 8001
	defaultTCPServerPort                 = 8002
	defaultIpData                        = true
	defaultTruncTextValues               = true
	defaultLogToStdout                   = true
	defaultLogFormat                     = LogFormatText
	defaultMaxInventorySize              = 1000 * 1000 // Size limit from Vortex collector service (1MB)
	defaultPayloadCompressionLevel       = 6           // default compression level used in go, higher than this does not show tangible benefits
	defaultPidFile                       = "/var/run/newrelic-infra/newrelic-infra.pid"
	defaultPluginActiveConfigsDir        = "integrations.d"
	defaultSelinuxEnableSemodule         = true
	defaultStartupConnectionTimeout      = "10s"
	defaultPartitionsTTL                 = "60s" // TTL for the partitions cache, to avoid polling continuously for them
	defaultStartupConnectionRetries      = 6     // -1 will try forever with an exponential backoff algorithm
	defaultSupervisorRpcSock             = "/var/run/supervisor.sock"
	defaultWinUpdatePlugin               = false
	defaultMetricsIngestEndpoint         = "/metrics"          // default: V1 endpoint root (/events/bulk), combine this with defaultCollectorURL
	defaultInventoryIngestEndpoint       = "/inventory"        // default: V1 endpoint root (/deltas, /deltas/bulk)
	defaultIdentityIngestEndpoint        = "/identity/v1"      // default: V1 endpoint root (/connect, /register/batch)
	defaultMetricsIngestV2Endpoint       = "/infra/v2/metrics" // default: V2 endpoint root (/events/bulk), combine this with defaultCollectorURL
	defaultFingerprintUpdateFreqSec      = 60                  // Default update freq of the fingerprint in seconds.
	defaultCloudMaxRetryCount            = 10
	defaultCloudRetryBackOffSec          = 60  // In seconds.
	defaultCloudMetadataExpiryInSec      = 300 // In seconds.
	defaultCloudMetadataDisableKeepAlive = true
	defaultRegisterConcurrency           = 4
	defaultRegisterBatchSize             = 100
	defaultRegisterFrequencySecs         = 15
	defaultProxyValidateCerts            = false
	defaultProxyConfigPlugin             = true
	defaultWinRemovableDrives            = true
	defaultTraces                        = []trace.Feature{trace.CONN}
	defaultMetricsMatcherConfig          = IncludeMetricsMap{}
	defaultRegisterMaxRetryBoSecs        = 60
)

// Default internal values
var (
	defaultAgentDir                string
	defaultConfigFiles             []string
	defaultLogFile                 string
	defaultNetworkInterfaceFilters map[string][]string
	defaultPassthroughEnvironment  []string
	defaultPluginConfigFiles       []string
	defaultPluginInstanceDir       string
	defaultConfigDir               string
	defaultLoggingConfigsDir       string
	defaultLoggingBinDir           string
	defaultFluentBitExe            string
	defaultFluentBitParsers        string
	defaultFluentBitNRLib          string
	defaultIntegrationsTempDir     string
)

func getDefaultFacterHomeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return usr.HomeDir, nil
}
