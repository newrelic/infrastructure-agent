// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"os/user"
)

const (
	// Plain text log format.
	LogFormatText = "text"
	// JSON log format.
	LogFormatJSON = "json"

	// Non configurable stuff
	defaultIdentityURLEu                 = "https://identity-api.eu.newrelic.com"
	defaultIdentityStagingURLEu          = "https://staging-identity-api.eu.newrelic.com"
	defaultCmdChannelURLEu               = "https://infrastructure-command-api.eu.newrelic.com"
	defaultCmdChannelStagingURLEu        = "https://staging-infrastructure-command-api.eu.newrelic.com"
	defaultCmdChannelURL                 = "https://infrastructure-command-api.newrelic.com"
	defaultCmdChannelStagingURL          = "https://staging-infrastructure-command-api.newrelic.com"
	defaultIdentityURL                   = "https://identity-api.newrelic.com"
	defaultIdentityStagingURL            = "https://staging-identity-api.newrelic.com"
	baseCollectorURL                     = "https://%sinfra-api.%snewrelic.com"
	baseDimensionalMetricURL             = "https://%smetric-api.%snewrelic.com"
	defaultSecureFederalURL              = "https://gov-infra-api.newrelic.com"
	defaultSecureFederalMetricURL        = "https://gov-metric-api.newrelic.com"
	defaultSecureFedralIdentityURL       = "https://gov-identity-api.newrelic.com"
	defaultSecureFedralCmdChannelURL     = "https://gov-infrastructure-command-api.newrelic.com"
	defaultAPMCollectorHost              = "collector.newrelic.com"
	defaultAPMCollectorHostEu            = "collector.eu.newrelic.com"
	defaultSecureFederalAPMCollectorHost = "gov-collector.newrelic.com"
	defaultAPMCollectorHostStaging       = "staging-collector.newrelic.com"
)

// Default configurable values
//
//nolint:gochecknoglobals,gomnd
var (
	// public
	DefaultContainerCacheMetadataLimit = 60
	DefaultDockerApiVersion            = "1.24" // minimum supported API by Docker 18.09.0
	DefaultDockerContainerdNamespace   = "moby"
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
	defaultInventoryArchiveEnabled       = true
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
	defaultStatusServerPort              = 8003
	defaultIpData                        = true
	defaultTruncTextValues               = true
	defaultLogToStdout                   = true
	defaultLogFormat                     = LogFormatText
	defaultLogLevel                      = LogLevelInfo
	defaultLogForward                    = false
	defaultLoggingAddCustomAtts          = false
	defaultLoggingDropAttFbInput         = false
	defaultLoggingRetryLimit             = "5"         // nolint:gochecknoglobals
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
	defaultDMIngestEndpoint              = "/metric/v1/infra"
	defaultMetricsIngestEndpoint         = "/metrics"          // default: V1 endpoint root (/events/bulk), combine this with defaultCollectorURL
	defaultInventoryIngestEndpoint       = "/inventory"        // default: V1 endpoint root (/deltas, /deltas/bulk)
	defaultIdentityIngestEndpoint        = "/identity/v1"      // default: V1 endpoint root (/connect, /register/batch)
	defaultMetricsIngestV2Endpoint       = "/infra/v2/metrics" // default: V2 endpoint root (/events/bulk), combine this with defaultCollectorURL
	defaultFingerprintUpdateFreqSec      = 60                  // Default update freq of the fingerprint in seconds.
	defaultCloudProvider                 = ""
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
	defaultIncludeMetricsMatcherConfig   = IncludeMetricsMap{}
	defaultExcludeMetricsMatcherConfig   = ExcludeMetricsMap{}
	defaultRegisterMaxRetryBoSecs        = 60
	defaultNtpPool                       = []string{} // i.e: []string{"time.cloudflare.com"}
	defaultNtpEnabled                    = false
	defaultNtpInterval                   = uint(15) // minutes
	defaultNtpTimeout                    = uint(5)  // seconds
	defaultProcessContainerDecoration    = true
)

// Default internal values
// nolint:gochecknoglobals
var (
	defaultAgentDir                string
	defaultSafeBinDir              string
	defaultConfigFiles             []string
	defaultLogFile                 string
	defaultNetworkInterfaceFilters map[string][]string
	defaultPassthroughEnvironment  []string
	defaultPluginConfigFiles       []string
	defaultPluginInstanceDir       string
	defaultConfigDir               string
	defaultLoggingConfigsDir       string
	defaultLoggingHomeDir          string
	defaultFluentBitParsers        string
	defaultFluentBitNRLib          string
	defaultIntegrationsTempDir     string
	defaultAgentTempDir            string
)

func getDefaultFacterHomeDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return usr.HomeDir, nil
}
