// Copyright 2023 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Contains all the bits and pieces we need to parse and manage
// the external configuration
//
//nolint:tagliatelle
package config

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/license"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/sirupsen/logrus"

	config_loader "github.com/newrelic/infrastructure-agent/pkg/config/loader"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	// TracesFieldName can be used as a field in the structured log.
	TracesFieldName      = "traces"
	TracesFieldComponent = "component"
	SupervisorTrace      = "supervisor"
	HttpTracer           = "HttpTracer"
	FeatureTrace         = "feature"
	ProcessTrace         = "process"

	// LogFilterWildcard will match everything.
	LogFilterWildcard = "*"

	envPrefix                 = "nria"
	ModeUnknown               = ""
	ModeRoot                  = "root"
	ModePrivileged            = "privileged"
	ModeUnprivileged          = "unprivileged"
	NonVerboseLogging         = 0
	VerboseLogging            = 1
	SmartVerboseLogging       = 2
	TroubleshootLogging       = 3
	TraceLogging              = 4
	TraceTroubleshootLogging  = 5
	defaultMemProfileInterval = 60 * 5
	agentTemporaryFolderName  = "tmp"
)

const (
	// LogLevelSmart keeps a limited cache of debug logs and output them only when an error happens.
	LogLevelSmart string = "smart"
	// LogLevelInfo agent will log in info level.
	LogLevelInfo  string = "info"
	LogLevelWarn  string = "warn"
	LogLevelError string = "error"
	// LogLevelDebug adds more vorbosity in the logs.
	LogLevelDebug string = "debug"
	// LogLevelTrace is the most verbose log level.
	LogLevelTrace string = "trace"
)

var (
	ErrUnableToParseConfigFile   = fmt.Errorf("unable to parse configuration file")
	ErrDatabindApply             = fmt.Errorf("databind error")
	ErrNoDatabindFound           = fmt.Errorf("no databind found")
	ErrNoDatabindSources         = fmt.Errorf("no databind sources provided")
	ErrUnexpectedVariablesAmount = fmt.Errorf("unexpected config file variables replacement amount")
	ErrUnexpectedVariablesType   = fmt.Errorf("unexpected config file variables replacement type")
	clog                         = log.WithComponent("Configuration")
)

type CustomAttributeMap map[string]interface{}

// IncludeMetricsMap configuration type to Map include_matching_metrics setting env var
type IncludeMetricsMap map[string][]string

// LogFilters configuration specifies which log entries should be included/excluded.
type LogFilters map[string][]interface{}

// Provider will retrieve the configuration.
// If changes will be required (e.g. refreshing) will be applied now.
type Provider interface {
	// Provide will retrieve the configuration.
	Provide() *Config
}

type DynamicConfig struct {
	databindSources        *databind.Sources
	templateConfigMetadata *config_loader.YAMLMetadata
	templateConfig         *Config
	refreshedConfig        *Config
}

func (dc *DynamicConfig) Provide() *Config {

	if dc.isEmpty() {
		return nil
	}

	if dc.isCached() && !dc.isExpired() {
		return dc.getCached()
	}

	var err error
	dc.refreshedConfig, err = applyDatabind(dc)

	if err != nil {
		clog.WithError(err).Debug("Config provider failed to apply databind")

		return nil
	}

	err = NormalizeConfig(dc.refreshedConfig, *dc.templateConfigMetadata)
	if err != nil {
		clog.WithError(err).Debug("Config provider failed to apply normalizer")

		return nil
	}

	return dc.refreshedConfig
}

func (dc *DynamicConfig) isExpired() bool {
	return dc.databindSources.GetSoonestTTL().Before(time.Now())
}

func (dc *DynamicConfig) getCached() *Config {
	return dc.refreshedConfig
}

func (dc *DynamicConfig) isCached() bool {
	return dc.refreshedConfig != nil
}

func (dc *DynamicConfig) isEmpty() bool {
	return dc.databindSources == nil
}

// IMPORTANT NOTE: If you add new config fields, consider checking the ignore list in
// the plugins/agent_config.go plugin to not send undesired fields as inventory
//
// Configuration structs
// Use the 'public' annotation to specify the visibility of the config option: false/obfuscate [default: true]
type Config struct {
	dynamicConfig *DynamicConfig `databind:"ignored"`

	// Databind provides varaiable (secrets, discovery) replacement capabilities for the configuration.
	Databind databind.YAMLAgentConfig `yaml:",inline" public:"false"`

	// License specifies the license key for your New Relic account. The agent uses this key to associate your server's
	// metrics with your New Relic account. This setting is created as part of the standard installation process.
	// Default: ""
	// Public: Yes
	License string `yaml:"license_key" envconfig:"license_key" public:"obfuscate"`

	// Fedramp use Fedramp endpoints. See https://docs.newrelic.com/docs/fedramp-endpoint-logs-metrics
	// Default: true
	// Public: No
	Fedramp bool `yaml:"fedramp" envconfig:"fedramp" public:"true"`

	// Staging is staging environment.
	// Default: false
	// Public: No
	Staging bool `yaml:"staging" envconfig:"staging" public:"false"`

	// CollectorURL is the base URL for the metrics and inventory ingest endpoints. See metrics and inventory
	// ingest endpoint configuration option.
	// Default: https://infra-api.newrelic.com
	// Public: No
	CollectorURL string `yaml:"collector_url" envconfig:"collector_url" public:"false"`

	// IdentityURL defines the base URL for the identity connect.
	// Default: https://infra-api.newrelic.com
	// Public: No
	IdentityURL string `yaml:"identity_url" envconfig:"identity_url" public:"false"`

	// MetricURL defines the url for the dimensional metric ingest endpoint
	// Default: https://metric-api.newrelic.com
	// Public: No
	MetricURL string `yaml:"metric_url" envconfig:"metric_url" public:"false"`

	// DMIngestEndpoint defines the API path for the infrastructure dimensional metric ingest endpoint
	// Default: /metric/v1/infra
	// Public: No
	DMIngestEndpoint string `yaml:"dm_endpoint" envconfig:"dm_endpoint" public:"false"`

	// CommandChannelURL defines the base URL for the command channel.
	// Default: https://infrastructure-command-api.newrelic.com
	// Public: No
	CommandChannelURL string `yaml:"command_channel_url" envconfig:"command_channel_url" public:"false"`

	// CommandChannelEndpoint is the suffix path for the command channel endpoint. The base URL is defined in the
	// config option as CommandChannelURL
	// Default: /agent_commands/v1/commands
	// Public: No
	CommandChannelEndpoint string `yaml:"command_channel_endpoint" envconfig:"command_channel_endpoint" public:"false"`

	// CommandChannelIntervalSec defines the polling interval for the command channel in seconds.
	// Default: https://infra-api.newrelic.com
	// Public: No
	CommandChannelIntervalSec int `yaml:"command_channel_interval_sec" envconfig:"command_channel_interval_sec" public:"false"`

	// IgnoreSystemProxy makes `HTTPS_PROXY` and `HTTP_PROXY` environment variables to be ignored, in case the Agent
	// requires to not using an existing system proxy, and connect directly to the New Relic metrics collector.
	// Default: False
	// Public: Yes
	IgnoreSystemProxy bool `yaml:"ignore_system_proxy" envconfig:"ignore_system_proxy"`

	// Proxy defines a proxy to communicate with New Relic. If so, set the proxy URL in the form
	// https://user:password@hostname:port. Can be HTTP or HTTPS.
	// Default: ""
	// Public: Yes
	Proxy string `yaml:"proxy" envconfig:"proxy"`

	// ProxyValidateCerts If set to true, when the proxy is configured to use an HTTPS connection, it will only work
	// when the HTTPS proxy has certificates from a valid Certificate Authority, or when the ca_bundle_file or
	// ca_bundle_dir configuration properties contain the HTTPS proxy certificates.
	// Default: False
	// Public: Yes
	ProxyValidateCerts bool `yaml:"proxy_validate_certificates" envconfig:"proxy_validate_certificates"`

	// ProxyConfigPlugin sends the following proxy configuration information as inventory:
	// `HTTPS_PROXY`
	// `HTTP_PROXY`
	// `proxy`
	// `ca_bundle_dir`
	// `ca_bundle_file`
	// `ignore_system_proxy`
	// `proxy_validate_certificates`
	// Default: True
	// Public: Yes
	ProxyConfigPlugin bool `yaml:"proxy_config_plugin" envconfig:"proxy_config_plugin"`

	// IgnoreReclaimable When true, the formulation of the host virtual memory considers SReclaimable as available
	// memory; otherwise SReclaimable will be considered part of the used memory.
	// Default: False
	// Public: Yes
	IgnoreReclaimable bool `yaml:"ignore_reclaimable" envconfig:"ignore_reclaimable"`

	// DisplayName overrides the auto-generated hostname for reporting. This is useful when you have multiple hosts
	// with the same name, since Infrastructure uses the hostname as the unique identifier for each host.
	// Keep in mind this value is also used for the loopback address replacement on entity names.
	// To be sure to understand how this entity name resolution works check the following link:
	// https://docs.newrelic.com/docs/integrations/integrations-sdk/file-specifications/integration-executable-file-specifications#h2-loopback-address-replacement-on-entity-namesç
	// Default: ""
	// Public: Yes
	DisplayName string `yaml:"display_name" envconfig:"display_name"`

	// DisableInventorySplit By default the agent splits the inventory data into small groups bounded by the value of
	// the config option MaxInventorySize; if this option is set to true, the inventory won't be splitted and the agent
	// will try to send it all in a single request.
	// Default: False
	// Public: No
	DisableInventorySplit bool `yaml:"disable_inventory_split" envconfig:"disable_inventory_split" public:"false"`

	// DnsHostnameResolution When true, the full hostname is resolved by performing a reverse lookup of the hosts
	// address; otherwise, it will be retrieved with the hostname command on Linux, and from the TCP/IP parameters of
	// the registry on Windows.
	// Default: True
	// Public: Yes
	DnsHostnameResolution bool `yaml:"dns_hostname_resolution" envconfig:"dns_hostname_resolution"`

	// DockerApiVersion specifies the Docker API Version to use for the Docker client.
	// Default: 1.24
	// Public: Yes
	DockerApiVersion string `yaml:"docker_api_version" envconfig:"docker_api_version"`

	// CustomAttributes is a list of custom attributes to annotate the data from this agent instance. Separate keys and
	// values with colons :, as in KEY: VALUE, and separate each key-value pair with a line break. Keys can be any
	// valid YAML except slashes /. Values can be any YAML string, including spaces.
	// Default: Empty
	// Public: Yes
	CustomAttributes CustomAttributeMap `yaml:"custom_attributes" envconfig:"custom_attributes"`

	// Verbose When verbose is set to 0, verbose logging is off, but the agent still creates logs. Set this to 1 to
	// create verbose logs to use in troubleshooting the agent. You can set this to 2 to use Smart Verbose Logs. Set to
	// 3 to forward debug logs to FluentBit. To enable log traces set this to 4, and to 5 to forward traces to FluentBit.
	// Default: 0
	// Public: Yes
	// Deprecated: use Log.Level instead.
	Verbose int `yaml:"verbose" envconfig:"verbose"`

	// The number of entries that will be cached in memory before being flushed (if an error has not been logged
	// beforehand).
	// Default: 1000
	// Public: Yes
	// Deprecated: use Log.SmartLevelEntryLimit instead.
	SmartVerboseModeEntryLimit int `yaml:"smart_verbose_mode_entry_limit" envconfig:"smart_verbose_mode_entry_limit"`

	// CPUProfile takes the path of a file that will be created and used to store profiling samples related to the CPU
	// usage of the agent in pprof format.
	// Default: ""
	// Public: No
	CPUProfile string `yaml:"cpu_profile" envconfig:"cpu_profile" public:"false"`

	// MemProfile takes the path of a file that will be created and used to store profiling samples related to memory consumption
	// usage of the agent in pprof format.
	// Default: ""
	// Public: No
	MemProfile string `yaml:"mem_profile" envconfig:"mem_profile" public:"false"`

	// Interval in seconds to create memory profile snapshots, default 300 seconds (5min)
	// Default: 300
	// Public: No
	MemProfileInterval int `yaml:"mem_profile_interval" envconfig:"mem_profile_interval" public:"false"`

	// WebProfile enables pprof profiler serving data via HTTP API
	// Default: false
	// Public: No
	WebProfile bool `yaml:"web_profile" envconfig:"web_profile" public:"false"`

	// StripCommandLine When true, the agent removes the command arguments from the 'commandLine' attribute of the
	// ProcessSample. This is a security measure to prevent leaking sensitive information.
	// Default: True
	// Public: Yes
	StripCommandLine bool `yaml:"strip_command_line" envconfig:"strip_command_line"`

	// OverrideHostname When set, this is the value that will be reported for the full hostname; otherwise,
	// the agent will perform the normal lookup behavior.
	// Default: ""
	// Public: Yes
	OverrideHostname string `yaml:"override_hostname" envconfig:"override_hostname"`

	// OverrideHostnameShort When set, this is the value that will be reported for the hostname; otherwise,
	// the agent will perform the normal lookup behavior.
	// Default: ""
	// Public: Yes
	OverrideHostnameShort string `yaml:"override_hostname_short" envconfig:"override_hostname_short"`

	// OverrideHostProc When set, this will change the base directory used when constructing paths for location
	// inside /proc/. This allows us to mock the filesystem in order to make tests.
	// Default: ""
	// Public: No
	OverrideHostProc string `yaml:"override_host_proc" envconfig:"override_host_proc" public:"false"`

	// OverrideHostSys When set, this will change the base directory used when constructing paths for location
	// inside /sys/. This allows us to mock filesystem in order to make tests.
	// Default: ""
	// Public: No
	OverrideHostSys string `yaml:"override_host_sys" envconfig:"override_host_sys" public:"false"`

	// OverrideHostEtc When set, this will change the base directory used when constructing paths for location
	// inside /etc/. This allows us to mock the filesystem in order to make tests.
	// Default: ""
	// Public: No
	OverrideHostEtc string `yaml:"override_host_etc" envconfig:"override_host_etc" public:"false"`

	// OverrideHostRoot When set, this will be use as a prefix when constructing paths for location inside the
	// /proc /sys and /etc directory. This allows us to mock the filesystem in order to make tests.
	// This config parameter is also used when building the Containerized Agent so it can read data from the
	// underlying host.
	// Setting this to '/my-root' it will make the agent construct paths like '/my-root/etc/my-file' when constructing
	// a path to the file 'my-file' stored in '/etc'. This allows us to mock filesystem in order to make tests.
	// Default: ""
	// Public: No
	OverrideHostRoot string `yaml:"overide_host_root" envconfig:"override_host_root" public:"false"`

	// IsContainerized identifies that the agent is running inside a container. This value is set through the
	// environment variable from the containerized agent Dockerfile at build time.
	// Default: False
	// Public: No
	IsContainerized bool `yaml:"is_containerized" envconfig:"is_containerized" public:"false"`

	// IsForwardOnly enables the forwarding mode, in this mode the agent doesn't activate any of its plugins or
	// samplers, and just forwards data from the integrations.
	// Default: False
	// Public: No
	IsForwardOnly bool `yaml:"is_forward_only" envconfig:"is_forward_only" public:"false"`

	// IsSecureForwardOnly has the same behaviour as `IsForwardOnly` but with some inventory data and a heartbeat
	// Default: False
	// Public: No
	IsSecureForwardOnly bool `yaml:"is_secure_forward_only" envconfig:"is_secure_forward_only" public:"false"`

	// K8sIntegration enables the K8sIntegrationSample, this sample returns the names of the integrations that
	// the agent has configured.
	// Default: False
	// Public: No
	K8sIntegration bool `yaml:"k8s_integration" envconfig:"k8s_integration" public:"false"`

	// AgentDir is the directory where the agent stores files like cache, inventory, integrations, etc.
	// Default (Linux): /var/db/newrelic-infra
	// Default (MacOS): /usr/local/var/db/newrelic-infra/
	// Default (Windows): C:\Program Files\NewRelic\newrelic-infra
	// Public: Yes
	AgentDir string `yaml:"agent_dir" envconfig:"agent_dir"`

	// ConfigDir is the main directory where the agent stores configs.
	// Default (Linux): /etc/newrelic-infra
	// Default (Windows): C:\Program Files\NewRelic\newrelic-infra
	// Public: Yes
	ConfigDir string `yaml:"config_dir" envconfig:"config_dir"`

	// Limits any length of the string metrics to 4095 characters.
	// Default: true
	// Public: yes
	TruncTextValues bool `yaml:"trunc_text_values" envconfig:"trunc_text_values"`

	// Change the log format. Current supported formats: json, common.
	// Default: text
	// Public: Yes
	// Deprecated: use Log.Format instead.
	LogFormat string `yaml:"log_format" envconfig:"log_format"`

	// LogFile defines the file path for the logs.
	// The agent standard installation creates a default log directory and it sets this filepath value in the
	// log_file configuration option for you.
	// Default (Linux): /var/log/newrelic-infra/newrelic-infra.log
	// Default (Windows): C:\Program Files\New Relic\newrelic-infra\newrelic-infra.log
	// Public: Yes
	// Deprecated: use Log.File instead.
	LogFile string `yaml:"log_file" envconfig:"log_file"`

	// Log is a map of custom logging configurations. Separate keys and values with colons :, as in KEY: VALUE, and
	// separate each key-value pair with a line break. Key-value can be any of the following:
	// "file: path/to/file.log" defines the log file path
	// "format: json" logging format (json, text)
	// "level: debug" logrus log level (error, warning, info, smart, debug, trace)
	// "forward: true" boolean to send logs to New Relic platform
	// "stdout: true" boolean to print logs to stdout
	// "smart_level_entry_limit: 50" number of entries that will be cached before being flushed (default: 1000)
	// "include_filters: " map entry to include the log entries with the defined fields (default: all log fields)
	// "exclude_filters: " map entry to exclude the log entries with the defined fields (default: none)
	// Default: none
	// Public: Yes
	Log LogConfig `yaml:"log" envconfig:"log"`

	// PidFile contains the location on Linux where the pid file of the agent process is created. It is used at startup
	// to ensure that no other instances of the agent are running.
	// Default: /var/run/newrelic-infra/newrelic-infra.pid
	// Public: Yes
	PidFile string `yaml:"pid_file" envconfig:"pid_file" os:"linux"`

	// MaxInventorySize sets the maximum size allowed for inventory data. If a plugin's inventory data exceeds this
	// value it will be dropped. Inventory deltas will be grouped in batches bounded by this value before being sent
	// to the NewRelic platform.
	// Default: 1000000 (1MB)
	// Public: No
	MaxInventorySize int `yaml:"max_inventory_size" envconfig:"max_inventory_size" public:"false"`

	// MaxProcs specifies the number of logical processors available to the agent. Increasing this value can help to
	// distribute the load between different cores. Default value is 1. If value is set to -1 then it will try to read
	// the environment variable GOMAXPROCS. If that variable is not set then the default value will be the total
	// number of cores available in the host.
	// Default: 1
	// Public: Yes
	MaxProcs int `yaml:"max_procs" envconfig:"max_procs"`

	// MetricsSystemSampleRate Sample rate of System Samples in seconds. Minimum value is 5. If value is -1 then
	// the sampler is disabled.
	// Default: 5
	// Public: Yes
	MetricsSystemSampleRate int `yaml:"metrics_system_sample_rate" envconfig:"metrics_system_sample_rate"`

	// MetricsStorageSampleRate Sample rate of Storage Samples in seconds. Minimum value is 5. If value is -1 then
	// the sampler is disabled.
	// Default: 20
	// Public: Yes
	MetricsStorageSampleRate int `yaml:"metrics_storage_sample_rate" envconfig:"metrics_storage_sample_rate"`

	// MetricsNetworkSampleRate Sample rate of Network Samples in seconds. Minimum value is 10. If value is -1 then
	// the sampler is disabled.
	// Default: 5
	// Public: Yes
	MetricsNetworkSampleRate int `yaml:"metrics_network_sample_rate" envconfig:"metrics_network_sample_rate"`

	// MetricsProcessSampleRate Sample rate of System Samples in seconds. Minimum value is 20. If value is -1 then
	// the sampler is disabled.
	// Default: 20
	// Public: Yes
	MetricsProcessSampleRate int `yaml:"metrics_process_sample_rate" envconfig:"metrics_process_sample_rate"`

	// HeartBeatSampleRate Interval in seconds for sending the HeartBeatSample.
	// Default: False
	// Public: No
	HeartBeatSampleRate int `yaml:"heart_beat_sample_rate" envconfig:"heart_beat_sample_rate" public:"false"`

	// DMSubmissionPeriod interval in seconds for triggering dimensional metrics submissions.
	// Default: False
	// Public: No
	DMSubmissionPeriod int `yaml:"dm_submission_period" envconfig:"dm_submission_period" public:"false"`

	// CustomSupportedFileSystems List of filesystems types the agent supports. This value should be a subset of the
	// default list, items that are not in the default list will be discarded.
	// Default: Empty
	// Public: Yes
	CustomSupportedFileSystems []string `yaml:"custom_supported_file_systems" envconfig:"custom_supported_filesystems"`

	// FileDevicesBlacklist List of storage devices to be ignored by the agent when gathering StorageSamples.
	// Default: Empty
	// Public: No
	// Deprecated: use FileDevicesIgnored instead.
	FileDevicesBlacklist []string `yaml:"file_devices_blacklist" envconfig:"file_devices_blacklist"`

	// FileDevicesIgnored List of storage devices to be ignored by the agent when gathering StorageSamples.
	// Default: Empty
	// Public: Yes
	FileDevicesIgnored []string `yaml:"file_devices_ignored" envconfig:"file_devices_ignored"`

	// NetworkInterfaceFilters You can use the network interface filters configuration to hide unused or uninteresting
	// network interfaces from the Infrastructure agent. This helps reduce resource usage, work, and noise in your data.
	// Default: Empty
	// Public: Yes
	NetworkInterfaceFilters map[string][]string `yaml:"network_interface_filters" envconfig:"network_interface_filters"`

	// IpData When true, IP addresses and MAC addresses will be fetched and added to the agent's connect fingerprint.
	// Default: True
	// Public: No
	IpData bool `yaml:"ip_data" envconfig:"ip_data" public:"false"`

	// CABundleFile If your https_proxy option references to a proxy with self-signed certificates, this option allows
	// you specify your proxy certificate file.
	// Default: ""
	// Public: Yes
	CABundleFile string `yaml:"ca_bundle_file" envconfig:"ca_bundle_file"`

	// CABundleDir If your https_proxy option references to a proxy with self-signed certificates, this option allows
	// you specify the directory where the proxy certificate is available.
	// The certificates in the directory must end with the .pem extension.
	// Default: ""
	// Public: Yes
	CABundleDir string `yaml:"ca_bundle_dir" envconfig:"ca_bundle_dir"`

	// SupervisorRpcSocket Location of the supervisor (http://supervisord.org/) socket.
	// Default: /var/run/supervisor.sock
	// Public: Yes
	SupervisorRpcSocket string `yaml:"supervisor_rpc_sock" envconfig:"supervisor_rpc_sock"`

	// SupervisorRefreshSec Sampling period / interval in seconds for Supervisor plugin
	// set as value -1 for disabling it, otherwise 10 is the minimum value
	// Default: 15
	// Public: Yes
	SupervisorRefreshSec int64 `yaml:"supervisor_interval_sec" envconfig:"supervisor_interval_sec"`

	// RpmRefreshSec Sampling period / interval in seconds for Rpm plugin. Set as value -1 for disabling it. 30 is
	// the minimum value. Only activated in root or privileged modes and on distros: RedHat, RedHat AWS and SUSE
	// Default: 30
	// Public: Yes
	RpmRefreshSec int64 `yaml:"rpm_interval_sec" envconfig:"rpm_interval_sec"`

	// DpkgRefreshSec Sampling period / interval in seconds for Dpkg plugin. Set as value -1 for disabling it.
	// 30 is the minimum value. Only activated in root or privileged modes and on debian based distros.
	// Default: 30
	// Public: Yes
	DpkgRefreshSec int64 `yaml:"dpkg_interval_sec" envconfig:"dpkg_interval_sec"`

	// DaemontoolsRefreshSec Sampling period / interval in seconds for Daemontools plugin. Set as value -1 for
	// disabling it. 10 is the minimum value
	// Default: 15
	// Public: Yes
	DaemontoolsRefreshSec int64 `yaml:"daemontools_interval_sec" envconfig:"daemontools_interval_sec"`

	// FacterIntervalSec Sampling period / interval in seconds for Facter plugin. Set as value -1 for disabling it,
	// otherwise 30 is the minimum value
	// Default: 30
	// Public: Yes
	FacterIntervalSec int64 `yaml:"facter_interval_sec" envconfig:"facter_interval_sec"`

	// FacterHomeDir sets the HOME environment variable for Facter (https://puppet.com/docs/facter). If unset,
	// it defaults to the current user's home directory.
	// Default: ""
	// Public: Yes
	FacterHomeDir string `yaml:"facter_home_dir" envconfig:"facter_home_dir"`

	// SelinuxIntervalSec Sampling period / interval in seconds for SELinux plugin. Set as value -1 for disabling it,
	// otherwise 30 is the minimum value. SELinux plugin is activated only in root mode.
	// This config option will be ignored if SelinuxEnableSemodule is set to false.
	// Default: 30
	// Public: Yes
	SelinuxIntervalSec int64 `yaml:"selinux_interval_sec" envconfig:"selinux_interval_sec"`

	// SelinuxEnableSemodule allows disabling `semodule -l`, which takes 100% CPU on some SELinux distributions
	// Default: True
	// Public: Yes
	SelinuxEnableSemodule bool `yaml:"selinux_enable_semodule" envconfig:"selinux_enable_semodule"`

	// SysctlFSNotify replaces previous Sysctl plugin using sample polling with FS-notify pub-sub mode.
	// Default: false
	// Public: Yes
	SysctlFSNotify bool `yaml:"sysctl_fs_notify" envconfig:"sysctl_fs_notify"`

	// SysctlIntervalSec Sampling period / interval in seconds for Sysctl plugin. Set as value -1 for disabling it.
	// 30 is the minimum value. This plugin can be activated only in root mode or privileged mode.
	// Default: 60
	// Public: Yes
	SysctlIntervalSec int64 `yaml:"sysctl_interval_sec" envconfig:"sysctl_interval_sec"`

	// SystemdIntervalSec Sampling period / interval in seconds for Systemd plugin. Set as value -1 for disabling it.
	// 10 is the minimum value.
	// Default: 30
	// Public: Yes
	SystemdIntervalSec int64 `yaml:"systemd_interval_sec" envconfig:"systemd_interval_sec"`

	// SysvInitIntervalSec Sampling period / interval in seconds for SysV plugin. Set as value -1 for disabling it.
	// 10 is the minimum value. This plugin can be activated only in root mode or privileged mode.
	// Default: 30
	// Public: Yes
	SysvInitIntervalSec int64 `yaml:"sysvinit_interval_sec" envconfig:"sysvinit_interval_sec"`

	// UpstartIntervalSec Sampling period / interval in seconds for Upstart plugin. Set as value -1 for disabling it.
	// 10 is the minimum value.
	// Default: 30
	// Public: Yes
	UpstartIntervalSec int64 `yaml:"upstart_interval_sec" envconfig:"upstart_interval_sec"`

	// NetworkInterfaceIntervalSec Sampling period / interval in seconds for NetworkInterface plugin. Set as value -1
	// for disabling it. 30 is the minimum value.
	// Default: 60
	// Public: Yes
	NetworkInterfaceIntervalSec int64 `yaml:"network_interface_interval_sec" envconfig:"network_interface_interval_sec"`

	// CloudSecurityGroupRefreshSec Sampling period / interval in seconds for CloudSecurityGroups plugin. Set as
	// value -1 for disabling it. 30 is the minimum value.
	// Default: 60
	// Public: Yes
	CloudSecurityGroupRefreshSec int64 `yaml:"cloud_security_group_refresh_sec" envconfig:"cloud_security_group_refresh_sec"`

	// KernelModulesRefreshSec Sampling period / interval in seconds for KernelModules plugin. Set as value -1
	// for disabling it. 10 is the minimum value.
	// Default: 10
	// Public: Yes
	KernelModulesRefreshSec int64 `yaml:"kernel_modules_refresh_sec" envconfig:"kernel_modules_refresh_sec"`

	// UsersRefreshSec Sampling period / interval in seconds for Users plugin. Set as value -1
	// for disabling it. 10 is the minimum value.
	// Default: 15
	// Public: Yes
	UsersRefreshSec int64 `yaml:"users_refresh_sec" envconfig:"users_refresh_sec"`

	// SshdConfigRefreshSec Sampling period / interval in seconds for Sshd plugin. Set as value -1
	// for disabling it. 10 is the minimum value.
	// Default: 15
	// Public: Yes
	SshdConfigRefreshSec int64 `yaml:"sshd_config_refresh_sec" envconfig:"sshd_config_refresh_sec"`

	// WindowsServicesRefreshSec Sampling period / interval in seconds for WindowsServices plugin. Set as value -1
	// for disabling it. 10 is the minimum value.
	// Default: 30
	// Public: Yes
	WindowsServicesRefreshSec int64 `yaml:"windows_services_refresh_sec" envconfig:"windows_services_refresh_sec" os:"windows"`

	// WindowsUpdatesRefreshSec Sampling period / interval in seconds for WindowsUpdates plugin. Set as value -1
	// for disabling it. 10 is the minimum value.
	// Default: 60
	// Public: Yes
	WindowsUpdatesRefreshSec int64 `yaml:"windows_updates_refresh_sec" envconfig:"windows_updates_refresh_sec" os:"windows"`

	// LogToStdout By default all logs are displayed in both standard output and a log file. If you want to disable
	// logs in the standard output you can set this configuration option to FALSE.
	// Default: True
	// Public: Yes
	// Deprecated: use Log.ToStdout instead.
	LogToStdout bool `yaml:"log_to_stdout" envconfig:"log_to_stdout"`

	// ContainerMetadataCacheLimit Time duration, in seconds, before expiring the cached containers metadata and
	// having to fetch it again.
	// Default: 60
	// Public: Yes
	ContainerMetadataCacheLimit int `yaml:"container_cache_metadata_limit" envconfig:"container_cache_metadata_limit"`

	// PayloadCompressionLevel sets the gzip compression level of the payload of the requests that the agent sends to
	// the backend: e.g. samples/deltas connect step info
	// HuffmanOnly=-2
	// NoCompression=0
	// BestSpeed=1
	// intermediate levels 2-8
	// BestCompression=9
	// Default: 6
	// Public: Yes
	PayloadCompressionLevel int `yaml:"payload_compression_level" envconfig:"payload_compression_level"`

	// PartitionsTTL Time duration to expire the cached list of storage partitions.
	// Default: 60s
	// Public: No
	PartitionsTTL string `yaml:"partitions_ttl" envconfig:"partitions_ttl" public:"false"`

	// StartupConnectionTimeout Time duration to wait before timing-out the request the agents makes at startup to
	// check the NewRelic platform availability. Used by defining reachability status of backend endpoints.
	// Default: 10s
	// Public: Yes
	StartupConnectionTimeout string `yaml:"startup_connection_timeout" envconfig:"startup_connection_timeout"`

	// StartupConnectionRetries Number of times the agent will retry the request to check the NewRelic platform
	// availability on startup before throwing an error. When set to a negative value, the agent will keep checking
	// the connection until it succeeds.
	// Default: 6
	// Public: Yes
	StartupConnectionRetries int `yaml:"startup_connection_retries" envconfig:"startup_connection_retries"`

	// FingerprintUpdateFreqSec Defines the frequency in seconds for the agent to reconnect and update the current
	// fingerprint with its assigned entity ID for the connect.
	// Default: 60
	// Public: No
	FingerprintUpdateFreqSec int `yaml:"fingerprint_update_freq" envconfig:"fingerprint_update_freq" public:"false"`

	// ForceProtocolV2toV3 Agent enables loopback-address replacement on the entity name (and therefor key)
	// automatically for v3 integration protocol. If you are using v2 for the integration protocol and you want
	// to have this behaviour then you can enable the entityname_integrations_v2_update option.
	// Default: False
	// Public: Yes
	ForceProtocolV2toV3 bool `yaml:"entityname_integrations_v2_update" envconfig:"entityname_integrations_v2_update"`

	// DisableAllPlugins disables all the plugins except does that send data required by
	// the platform team. Can be overridden per plugin by setting the
	// `<Plugin>IntervalSec` config options to a value greater than
	// `FREQ_DISABLE_SAMPLING` and different than `FREQ_DEFAULT_SAMPLING`.
	// Default: False
	// Public: Yes
	DisableAllPlugins bool `yaml:"disable_all_plugins" envconfig:"disable_all_plugins"`

	// EventQueueDepth We use two queues to send the events to metrics digest: (event -> eventQueue -> batch ->
	// batchQueue -> HTTP post). This config option allow us to increase the eventQueue size before accumulate these
	// events in batches. Using this approach we minimize the impact of high-latency HTTP calls. If HTTP calls are
	// slow, we'll still be able to run the event queue receiver and accumulate a reasonable number of batches before
	// we fill up on batches as well.
	// Default: 1000
	// Public: No
	EventQueueDepth int `yaml:"event_queue_depth" envconfig:"event_queue_depth" public:"false"` // See event_sender.go

	// BatchQueueDepth We use two queues to send the events to metrics digest: (event -> eventQueue -> batch ->
	// batchQueue -> HTTP post). This config option allow us to increase the batchQueue size.
	// Default: 200
	// Public: No
	BatchQueueDepth int `yaml:"batch_queue_depth" envconfig:"batch_queue_depth" public:"false"` // See event_sender.go

	// InventoryQueueLen sets the inventory processing queue size. Zero value makes inventory processing synchronous (blocking call).
	// Default: 0
	// Public: Yes
	InventoryQueueLen int `yaml:"inventory_queue_len" envconfig:"inventory_queue_len" public:"true"`

	// AsyncInventoryHandlerEnabled when set to true, enables the inventory handler that parallelize processing that allows handling larger inventory payloads.
	// Default: false
	// Public: no
	AsyncInventoryHandlerEnabled bool `yaml:"async_inventory_handler_enabled" envconfig:"async_inventory_handler_enabled" public:"false"`

	// EnableWinUpdatePlugin enables the windows updates plugin which retrieves the lists of hotfix that are installed
	// on the host.
	// Default: False
	// Public: Yes
	EnableWinUpdatePlugin bool `yaml:"enable_win_update_plugin" envconfig:"enable_win_update_plugin" os:"windows"`

	// InventoryArchiveEnabled When enabled, the delta storage will save each successful deltas submission into
	// .sent files in the delta store.
	// Default: True
	// Public: True
	InventoryArchiveEnabled bool `yaml:"inventory_archive_enabled" envconfig:"inventory_archive_enabled" public:"true"`

	// CompactEnabled When enabled, the delta storage will be compacted after its storage directory surpasses a
	// certain threshold set by the CompactTreshold options.	Compaction works by removing the data of inactive plugins
	// and the archived deltas of the active plugins; archive deltas are deltas that have already been sent to the
	// NewRelic platform.
	// Default: True
	// Public: No
	CompactEnabled bool `yaml:"compaction_enabled" envconfig:"compaction_enabled" public:"false"`

	// CompactThreshold Size in bytes to use as threshold for executing the delta storage compaction when the
	// CompactEnabled config option is set to true.
	// Default: 20971520 (20 MB)
	// Public: No
	CompactThreshold uint64 `yaml:"compaction_threshold" envconfig:"compaction_threshold" public:"false"`

	// IgnoredInventoryPaths is not a configurable option. It maps the values from ignored_inventory config option
	// Default: Empty
	// Public: No
	IgnoredInventoryPaths []string `yaml:"ignored_inventory" envconfig:"ignored_inventory" public:"false"`

	// WhitelistProcessSample only collects process samples for processes we care about, this is a WINDOWS ONLY CONFIG
	// Default: Empty
	// Public: No
	// Deprecated: use AllowedListProcessSample instead.
	WhitelistProcessSample []string `yaml:"whitelist_process_sample" envconfig:"whitelist_process_sample" public:"false"`

	// AllowedListProcessSample only collects process samples for processes we care about, this is a WINDOWS ONLY CONFIG
	// Default: Empty
	// Public: No
	// Deprecated: use IncludeMatchingMetrics instead.
	AllowedListProcessSample []string `yaml:"allowed_list_process_sample" envconfig:"allowed_list_process_sample" public:"false"`

	// DisableWinSharedWMI uses shared WMI if possible, fixed leaks on Win10/Server 2016 and newer
	// Default: False
	// Public: No
	DisableWinSharedWMI bool `yaml:"disable_win_shared_wmi" envconfig:"disable_win_shared_wmi" public:"false"`

	// DisableZeroRSSFilter Set to true to turn off ProcessSample filtering of 0 RSS processes. May have performance impact.
	// Default: False
	// Public: No
	DisableZeroRSSFilter bool `yaml:"disable_zero_mem_process_filter" envconfig:"disable_zero_mem_process_filter" public:"false"`

	// EnableElevatedProcessPriv Set to true on Windows to activate SeDebugPrivilege use for Process Info
	// Default: False
	// Public: No
	EnableElevatedProcessPriv bool `yaml:"enable_elevated_process_priv" envconfig:"enable_elevated_process_priv" public:"false"`

	// EnableWmiProcData Set to true to get process info from WMI and skip query access check
	// Default: False
	// Public: No
	EnableWmiProcData bool `yaml:"enable_wmi_proc_data" envconfig:"enable_wmi_proc_data" public:"false"`

	// OfflineTimeToReset If the cached inventory becomes older than this time (because e.g. the agent is offline),
	// it is reset
	// Default: 24h
	// Public: No
	OfflineTimeToReset string `yaml:"offline_time_to_reset" envconfig:"offline_time_to_reset" public:"false"`

	// Features enables features that could also be enabled via command-api FF.
	// Default: Empty
	// Public: No
	Features map[string]bool `yaml:"features" envconfig:"features" public:"false"`

	// RegisterConcurrency Amount of workers sending parallel requests for entity registration
	// Default: 4
	// Public: No
	RegisterConcurrency int `yaml:"register_concurrency" envconfig:"register_concurrency" public:"false"`

	// RegisterBatchSize Amount of remote entities per register request
	// Default: 100
	// Public: No
	RegisterBatchSize int `yaml:"register_batch" envconfig:"register_batch" public:"false"`

	// RegisterFrequencySecs Frequency for register request to be sent if batch size is not reached
	// Default: 15
	// Public: No
	RegisterFrequencySecs int `yaml:"register_freq_secs" envconfig:"register_freq_secs" public:"false"`

	// CustomPluginInstallationDir Specify a custom path to install integrations. The difference is that with this
	// allows to install integrations outside the agent_dir. It has the first priority when the agent is looking for
	// installed integrations.
	// Default: ""
	// Public: No
	CustomPluginInstallationDir string `yaml:"custom_plugin_installation_dir" envconfig:"custom_plugin_installation_dir" public:"false"`

	// PluginDir Directory containing integrations configuration files of the integrations. Each integration has his
	// own configuration file, named by default <integration_name>-config.yml, placed in a predefined location from
	// which the agent will load on initialization.
	// Default (Linux): /etc/newrelic-infra/integrations.d/
	// Default (Windows): C:\Program Files\New Relic\newrelic-infra\integrations.d
	// Public: Yes
	PluginDir string `yaml:"plugin_dir" envconfig:"plugin_dir"`

	// PassthroughEnvironment A list of environment variables that will be passed to all integrations. Regular
	// expressions can also be provided to match one or multiple environment variables. If an integration already has
	// an existing configuration option with the same name, then the environment variable
	// takes precedence.
	// Default: Empty
	// Public: Yes
	PassthroughEnvironment []string `yaml:"passthrough_environment" envconfig:"passthrough_environment"`

	// PluginConfigFiles This configuration parameter specify the agent to look for newrelic-infra-plugins.yml
	// Default: Empty
	// Public: No
	PluginConfigFiles []string `envconfig:"ignored" public:"false"`

	// PluginInstanceDirs Internal setting, no documentation
	// Default: Empty
	// Public: No
	PluginInstanceDirs []string `envconfig:"ignored" public:"false"`

	// LoggingConfigsDir folder containing configuration files for the log forwarder.
	// Default: /etc/newrelic-infra/logging.d
	// Public: Yes
	LoggingConfigsDir string `yaml:"logging_configs_dir" envconfig:"logging_configs_dir" public:"true"`

	// LoggingBinDir folder containing binaries for the log forwarder.
	// Default: /var/db/newrelic-infra/newrelic-integrations/logging/
	// Public: No
	LoggingBinDir string `yaml:"logging_bin_dir" envconfig:"logging_bin_dir" public:"false"`

	// LoggingHomeDir folder containing plugins and other required files for the log forwarder.
	// Default (Linux): /var/db/newrelic-infra/newrelic-integrations/logging/
	// Default (Windows): C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\
	// Public: No
	LoggingHomeDir string `yaml:"logging_home_dir" envconfig:"logging_home_dir" public:"false"`

	// LoggingRetryLimit determines the value of the Retry_Limit for the New Relic fluent-bit output plugin.
	// https://github.com/newrelic/newrelic-fluent-bit-output/blob/7cbb4393aa36e48bad783231182f707037ebf217/README.md#retry-logic
	// Default: "5"
	// Public: Yes
	LoggingRetryLimit string `yaml:"logging_retry_limit" envconfig:"logging_retry_limit" public:"true"`

	// FluentBitExePath is the location from where the agent can execute fluent-bit.
	// Default (Linux): /opt/td-agent-bit/bin/td-agent-bit
	// Default (Windows): C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\fluent-bit
	// Public: No
	FluentBitExePath string `yaml:"fluent_bit_exe_path" envconfig:"fluent_bit_exe_path" public:"false"`

	// FluentBitParsersPath is the location where the FluentBit parsers.conf file is placed. It is currently required
	// by the "syslog" input plugin, specifies several message parsers and comes out-of-the-box with FluentBit.
	// Default: /var/db/newrelic-infra/newrelic-integrations/logging/parsers.conf
	// Public: No
	FluentBitParsersPath string `yaml:"fluent_bit_parsers_path" envconfig:"fluent_bit_parsers_path" public:"false"`

	// FluentBitNRLibPath is the location from where fluent-bit can load the newrelic fluent-bit library.
	// Default: /var/db/newrelic-infra/newrelic-integrations/logging/out_newrelic.so
	// Public: No
	FluentBitNRLibPath string `yaml:"fluent_bit_nr_lib_path" envconfig:"fluent_bit_nr_lib_path" public:"false"`

	// HTTPServerEnabled By setting true this configuration parameter (used by statsD integration v1) the agent will
	//	// open HTTP port (by default, 8001) to receive integration payloads via HTTP.
	// Default: False
	// Public: Yes
	HTTPServerEnabled bool `yaml:"http_server_enabled" envconfig:"http_server_enabled"`

	// HTTPServerHost By setting this value (used only by statsD integration) the agent will start listening on the
	// HTTPServerPort for receiving data from New Relic statsD backend.
	// Default: localhost
	// Public: Yes
	HTTPServerHost string `yaml:"http_server_host" envconfig:"http_server_host"`

	// HTTPServerPort Set the port for http server (used by statsD integration) to receive integration payloads.
	// Default: 8001
	// Public: Yes
	HTTPServerPort int `yaml:"http_server_port" envconfig:"http_server_port"`

	// HTTPServerCert Path to a PEM-encoded certificate to listen for integration payloads over HTTPs.
	HTTPServerCert string `yaml:"http_server_cert" envconfig:"http_server_cert"`

	// HTTPServerCert Path to a PEM-encoded key to listen for integration payloads over HTTPs.
	HTTPServerKey string `yaml:"http_server_key" envconfig:"http_server_key"`

	// HTTPServerCert Path to a PEM-encoded CA certificate to enforce client certificate validation for HTTPs requests.
	HTTPServerCA string `yaml:"http_server_ca" envconfig:"http_server_ca"`

	// TCPServerEnabled By setting true this configuration parameter (used by statsD integration v1) the agent will
	// open an TCP port (by default, 8002) to receive integration payloads via TCP.
	// Default: False
	// Public: Yes
	TCPServerEnabled bool `yaml:"tcp_server_enabled" envconfig:"tcp_server_enabled"`

	// TCPServerPort Set the port for tcp server to receive integration payloads.
	// Default: 8002
	// Public: Yes
	TCPServerPort int `yaml:"tcp_server_port" envconfig:"tcp_server_port"`

	// StatusServerEnabled will listen into TCP port (status_server_port) to serve status requests.
	// Default: False
	// Public: Yes
	StatusServerEnabled bool `yaml:"status_server_enabled" envconfig:"status_server_enabled"`

	// StatusServerPort Set the port for status server.
	// Default: 8003
	// Public: Yes
	StatusServerPort int `yaml:"status_server_port" envconfig:"status_server_port"`

	// StatusServerPort Set the port for status server.
	// Default: IdentityURL, CommandChannelURL, MetricsIngestURL, InventoryIngestURL
	// Public: Yes
	StatusEndpoints []string `yaml:"status_endpoints" envconfig:"status_endpoints"`

	// AppDataDir This option is only for Windows. It defines the path to store data in a different path than the
	// program files directory.
	// - %AppDir%/data: used for storing the delta data.
	// - %AppDir%/user_data: external directory for user-generated json files.
	// - %AppDir%/newrelic-infra.log: If log file config option is not defined, then we use this directory path
	// as default.
	// Default: env(ProgramData)\New Relic\newrelic-infra
	// Public: Yes
	AppDataDir string `yaml:"app_data_dir" envconfig:"app_data_dir" os:"windows"`

	// DisableCloudMetadata disables cloud metadata collection. If the agent is running	in a cloud instance, the Agent
	// will try to detect the cloud type and it will fetch metadata like: instanceID, instanceType,
	// cloudSource, hostType, etc.
	// Default: False
	// Public: Yes
	DisableCloudMetadata bool `yaml:"disable_cloud_metadata" envconfig:"disable_cloud_metadata"`

	// DisableCloudInstanceId is similar as DisableCloudMetadata, but DisableCloudInstanceId disables
	// cloud metadata collection only for host alias plugin
	// Default: False
	// Public: Yes
	DisableCloudInstanceId bool `yaml:"disable_cloud_instance_id" envconfig:"disable_cloud_instance_id"`

	// CloudProvider This sets the cloud provider the agent is running in. When this is set up, the agent will wait
	// until it has acquired the instance ID from the cloud provider before submitting any data to the backend.
	// Default: ""
	// Allowed values: aws, azure, gcp, alibaba
	// Public: Yes
	CloudProvider string `yaml:"cloud_provider" envconfig:"cloud_provider"`

	// CloudMaxRetryCount If the agent is running in a cloud instance, the agent will try to detect the	cloud type and
	// it will fetch metadata like: instanceID, instanceType, cloudSource, hostType.
	// This configuration parameter sets the number of retries in case that cloud detection failed. If during the agent
	// initialization the cloud detection fails it will retry after waiting for  CloudRetryBackOffSec.
	// Default: 10
	// Public: Yes
	CloudMaxRetryCount int `yaml:"cloud_max_retry_count" envconfig:"cloud_max_retry_count"`

	// CloudRetryBackOffSec This configuration parameter set the number of seconds delay between the cloud detection
	// retries in case that cloud detection failed. If during the agent initialization the cloud detection fails	it
	// will retry for CloudMaxRetryCount times.
	// Default: 60
	// Public: Yes
	CloudRetryBackOffSec int `yaml:"cloud_retry_backoff_sec" envconfig:"cloud_retry_backoff_sec"`

	// RegisterMaxRetryBoSecs This configuration parameter set the number of seconds delay between the
	// retries in case that entity registration fails.
	// Default: 60
	// Public: Yes
	RegisterMaxRetryBoSecs int `yaml:"register_max_retry_backoff_sec" envconfig:"register_max_retry_backoff_sec"`

	// CloudMetadataExpiryInSec If the agent is running in a cloud instance, the agent will try to detect the cloud
	// type and it will fetch metadata like: instanceID, instanceType, cloudSource, hostType. This configuration
	// parameter sets the interval of time on which the	metadata should be expired and re-fetched.
	// Default: 300
	// Public: Yes
	CloudMetadataExpiryInSec int `yaml:"cloud_metadata_expiry_sec" envconfig:"cloud_metadata_expiry_sec"`

	// CloudMetadataDisableKeepAlive If the agent is running in a cloud instance, the agent will try to detect the cloud
	// type and it will fetch metadata like: instanceID, instanceType, cloudSource, hostType. This configuration
	// parameter sets HTTP Connection header to close when querying the Cloud provider metadata.
	// Default: true
	// Public: Yes
	CloudMetadataDisableKeepAlive bool `yaml:"cloud_metadata_disable_keep_alive" envconfig:"cloud_metadata_disable_keep_alive"`

	// RemoveEntitiesPeriod Defines the frequency to engage the process of deleting entities that haven't been reported
	// information during the frequency interval. Valid time units are: "s" (seconds), "m" (minutes), "h" (hour).
	// Default: 48h
	// Public: Yes
	RemoveEntitiesPeriod string `yaml:"remove_entities_period" envconfig:"remove_entities_period"`

	// MetricsIngestEndpoint is the path for metrics ingest endpoint. The base URL is defined in the config option
	// collector URL.
	// Default: /infra/v2/metrics
	// Public: No
	MetricsIngestEndpoint string `yaml:"metrics_ingest_endpoint" envconfig:"metrics_ingest_endpoint" public:"false"`

	// InventoryIngestEndpoint This is the path for inventory ingest endpoint. The base URL is defined in the config
	// option collector URL.
	// Default: /inventory
	// Public: No
	InventoryIngestEndpoint string `yaml:"inventory_ingest_endpoint" envconfig:"inventory_ingest_endpoint" public:"false"`

	// IdentityIngestEndpoint This is the suffix path for identity connect endpoint. The base URL is defined in the
	// config option as identity url.
	// Default: /identity/v1
	// Public: No
	IdentityIngestEndpoint string `yaml:"identity_ingest_endpoint" envconfig:"identity_ingest_endpoint" public:"false"`

	// MaxMetricsBatchSizeBytes Defined Batch size in bytes for the events sent to metric-ingest. See batch_queue_depth
	// for more information.
	// Default: 1000000
	// Public: No
	MaxMetricsBatchSizeBytes int `yaml:"max_metrics_batch_size_bytes" envconfig:"max_metrics_batch_size_bytes" public:"false"`

	// MaxMetricBatchEntitiesCount Defined a max amount of entities to be submitted in a single metric-ingest request. Used to avoid reach max size in req Header.
	// Default: 300
	// Public: No
	MaxMetricBatchEntitiesCount int `yaml:"max_metrics_batch_entities_count" envconfig:"max_metrics_batch_entities_count" public:"false"`

	// MaxMetricBatchEntitiesQueue Defined a max amount of queued entities to be submited. Used to avoid memory consumption if metrics could not be submitted.
	// Default: 1000
	// Public: No
	MaxMetricBatchEntitiesQueue int `yaml:"max_metrics_batch_entities_queue" envconfig:"max_metrics_batch_entities_queue" public:"false"`

	// ConnectEnabled It enables or disables the connect for the agent ID resolution given the agent fingerprint.
	// If the config option is enabled it also reconnects to update the fingerprint with the given agent ID.
	// In case this config is enabled then it adds the resolved agent ID in the header as X-NRI-Agent-Entity-Id.
	// Default: False
	// Public: No
	ConnectEnabled bool `yaml:"connect_enabled" envconfig:"connect_enabled" public:"false"`

	// RegisterEnabled If it's enabled the register sends the entities returned by the integrations and it assigns
	// these entities with an assigned entity ID.
	// Default: False
	// Public: No
	RegisterEnabled bool `yaml:"register_enabled" envconfig:"register_enabled" public:"false"`

	// FilesConfigOn enables or disables the configuration file monitoring. Disabled by default. We just keep this
	// configuration value for backwards compatibilities, but any new agent should enable this value.
	// Default: False
	// Public: No
	FilesConfigOn bool `yaml:"files_config_enabled" envconfig:"files_config_enabled" public:"false"`

	// DebugLogSec Value in seconds. It defines the frequency we report the memory stats
	// Default: 600
	// Public: No
	DebugLogSec int `yaml:"debug_log_sec" envconfig:"debug_log_sec" public:"false"`

	// OfflineLoggingMode If it's enabled deltas from the plugins won't be sent.
	// Environment: INFRASTRUCTURE_OFFLINE_MODE (instead of boolean uses value 1 for enabling offline logging mode)
	// Default: False
	// Public: No
	OfflineLoggingMode bool `envconfig:"ignored" public:"false"`

	// WinProcessPriorityClass Only for windows: This configuration option allows increasing the newrelic-infra.exe
	// process priority to any of the following values: Normal, Idle, High, RealTime, BelowNormal, AboveNormal
	// Default: ""
	// Public: Yes
	WinProcessPriorityClass string `yaml:"win_process_priority_class" envconfig:"win_process_priority_class" os:"windows"`

	// WinRemovableDrives enables the Windows Agent to report drives `A:` and `B:` when they are mapped to removable
	// drives.
	// Default: True
	// Public: Yes
	WinRemovableDrives bool `yaml:"win_removable_drives" envconfig:"win_removable_drives" os:"windows"` // enables removable drives in storage sampler

	// LegacyStorageSampler Setting this value to true will force the agent to use windows WMI (the legacy method of
	// the Agent to grab metrics for Windows: e.g StorageSampler) and disable the new method which is using PDH library
	// Default (amd64): False
	// Default (386): True
	// Public: Yes
	LegacyStorageSampler bool `yaml:"legacy_storage_sampler" envconfig:"legacy_storage_sampler" os:"windows"`

	// RunMode It can be one of `root`, `privileged` or `unprivileged`. The value cannot be manually set, it's taken
	// from the runtime environment following the next heuristic:
	// - If the user running the agent is the `root` user, then the mode is `root`. This is the only available mode for the agent when running on Windows.
	// - If the user is other than `root` and the agent binary contains the following capabilities `cap_dac_read_search` and `cap_sys_ptrace` then the mode is `privileged`.
	// - If the user is other than `root` but the capabilities don't match the ones in the previous rule, then the mode is `unprivileged`.
	// Default: Runtime value
	// Public: No
	RunMode string

	// AgentUser The name of the user that's executing the agent process. This value is taken from the runtime
	// environment and cannot be manually set. The default Linux installation uses by default the `root` account to run
	// the agent, this can be changed using the `privileged` and `unprivileged` runmodes. In Windows the
	// `NT AUTHORITY\SYSTEM` account is used when the service is created by the MSI installer.
	// Default: Runtime value
	// Public: No
	AgentUser string

	// ExecutablePath The executable path of the agent process, this value is taken from the runtime environment and
	// cannot be manually set.
	// Default: Runtime value
	// Public: No
	ExecutablePath string

	// FirstReapInterval Defines how much do we have to wait for the first reap.
	// Default: 1s
	// Public: No
	FirstReapInterval time.Duration

	// ReapInterval Defines the frequency for the reaping process. In the reaping process we update the inventory
	// cached.
	// Default: 10s
	// Public: No
	ReapInterval time.Duration

	// SendInterval Defines the frequency to send the deltas. In case there is an error we use an exponential
	// backoff retry
	// Default: 10s
	// Public: No
	SendInterval time.Duration

	// IgnoredInventoryPathsMap It's not a configurable option. It maps the values from ignored_inventory config option
	// Default: Runtime value
	// Public: No
	IgnoredInventoryPathsMap map[string]struct{}

	// K8sIntegrationSamplesIntervalSec Interval for emitting samples defining which integrations are running for the
	// current pod when running inside a sidecar.
	// Default: 30
	// Public: No
	K8sIntegrationSamplesIntervalSec int64 `yaml:"k8s_integration_samples_interval_sec" envconfig:"k8s_integration_samples_interval_sec" public:"false"`

	// MetricsNFSSampleRate Sample rate of NFS Storage Samples in seconds. Minimum value is 5. If value is -1 then
	// the sampler is disabled.
	// Default: 20
	// Public: Yes
	MetricsNFSSampleRate int `yaml:"metrics_nfs_sample_rate" envconfig:"metrics_nfs_sample_rate"`

	// DetailedNFS when true will provide a complete list of NFS metrics.
	// Default: False
	// Public: Yes
	DetailedNFS bool `yaml:"detailed_nfs" envconfig:"detailed_nfs"`

	// Internals

	// concurrency support
	lock sync.Mutex

	// this is the default "persister" folder that the SDK uses. right now we don't allow configuration but we could at some point
	// send this to the integrations for them to use for persisting data.
	DefaultIntegrationsTempDir string

	// EnableProcessMetrics enables/disables process metrics, it does not enforce when not set.
	// Default: empty
	// Public: Yes
	EnableProcessMetrics *bool `yaml:"enable_process_metrics" envconfig:"enable_process_metrics"`

	// IncludeMetricsMatchers Configuration of the metrics matchers that determine which metric data should the agent
	// send to the New Relic backend.
	// If no configuration is defined, the previous behaviour is maintained, i.e., every metric data captured is sent.
	// If a configuration is defined, then only metric data matching the configuration is sent.
	// Note that ALL DATA NOT MATCHED WILL BE DROPPED.
	// Also note that at present it ONLY APPLIES to metric data related to processes. All other metric data is still being sent as usual.
	// Default: none
	// Public: Yes
	IncludeMetricsMatchers IncludeMetricsMap `yaml:"include_matching_metrics" envconfig:"include_matching_metrics"`

	// AgentMetricsEndpoint Set the endpoint (host:port) for the HTTP server the agent will use to server OpenMetrics
	// if empty the server will be not spawned
	// Default: empty
	// Public: Yes
	AgentMetricsEndpoint string `yaml:"agent_metrics_endpoint" envconfig:"agent_metrics_endpoint"`

	// SelfInstrumentation Set the agent self instrumentation to be used. Valid values: newrelic
	// if empty the agent will not be self instrumented
	// Default: empty
	// Public: No
	SelfInstrumentation string `yaml:"self_instrumentation" envconfig:"self_instrumentation"`

	// SelfInstrumentationApmHost defines the url for APM collector host
	// Default: collector.newrelic.com
	// Public: No
	SelfInstrumentationApmHost string `yaml:"self_instrumentation_apm_host" envconfig:"self_instrumentation_apm_host"`

	// SelfInstrumentationTelemetryEndpoint OpenTelemetry endpoint for self instrumentation
	// Default: empty
	// Public: No
	SelfInstrumentationTelemetryEndpoint string `yaml:"self_instrumentation_telemetry_endpoint" envconfig:"self_instrumentation_telemetry_endpoint"`

	// NtpMetrics is a map for ntp configuration. It is disabled by default.
	// Separate keys and values with colons :, as in KEY: VALUE, and separate each key-value pair with a line break.
	// Key-value can be any of the following:
	// "enabled: boolean" flag to enable/disable the ntp values (Default: false)
	// "pool: []string" list of ntp servers (Default: [])
	// "interval: int" interval in minutes to check ntp servers  (Default: 15)
	// "timeout: int" ntp request timeout value in seconds (Default: 10)
	// Default: none
	// Public: Yes
	NtpMetrics NtpConfig `yaml:"ntp_metrics" envconfig:"ntp_metrics"`

	// Http allows specifying extra configuration for the http client.
	// e.g. adding proxy headers.
	// Default: none
	// Public: Yes
	Http HttpConfig `yaml:"http" envconfig:"http"`

	// AgentTempDir is the directory where the agent stores temporary files (i.e. fb config, discovery...)
	// It will be DELETED on every agent restart only if it matches default value
	//
	// Default (Linux): /var/db/newrelic-infra/tmp
	// Default (MacOS AMD): /usr/local/var/db/newrelic-infra/tmp
	// Default (MacOS ARM): /opt/homebrew/var/db/newrelic-infra/tmp
	// Default (Windows): C:\ProgramData\New Relic\newrelic-infra\tmp
	// Public: no
	AgentTempDir string `yaml:"-" envconfig:"-"`
}

// KeyValMap is used whenever a key value pair configuration is required.
type KeyValMap map[string]string

// HttpConfig is the configuration to unmarshal http custom configuration.
type HttpConfig struct {
	Headers KeyValMap `yaml:"headers" envconfig:"headers"`
}

// NewHttpConfig returns a new instance of HttpConfig.
func NewHttpConfig() HttpConfig {
	return HttpConfig{
		Headers: make(KeyValMap),
	}
}

// Troubleshoot trobleshoot mode configuration.
type Troubleshoot struct {
	Enabled      bool
	AgentLogPath string
}

// NewTroubleshootCfg creates a troubleshooting mode config.
func NewTroubleshootCfg(isTroubleshootMode, agentLogsToFile bool, agentLogFile string) Troubleshoot {
	t := Troubleshoot{
		Enabled: isTroubleshootMode,
	}

	if agentLogsToFile {
		t.AgentLogPath = agentLogFile
	}

	return t
}

// LogConfig map all logging configuration options
type LogConfig struct {
	File                 string `yaml:"file" envconfig:"file"`
	Level                string `yaml:"level" envconfig:"level"`
	Format               string `yaml:"format" envconfig:"format"`
	Forward              *bool  `yaml:"forward,omitempty" envconfig:"forward"`
	ToStdout             *bool  `yaml:"stdout,omitempty" envconfig:"stdout"`
	SmartLevelEntryLimit *int   `yaml:"smart_level_entry_limit,omitempty" envconfig:"smart_level_entry_limit"`

	IncludeFilters LogFilters `yaml:"include_filters" envconfig:"include_filters"`
	ExcludeFilters LogFilters `yaml:"exclude_filters" envconfig:"exclude_filters"`

	Rotate LogRotateConfig `yaml:"rotate" envconfig:"rotate"`
}

func NewLogConfig() *LogConfig {
	return &LogConfig{
		Level:                defaultLogLevel,
		File:                 defaultLogFile,
		Format:               defaultLogFormat,
		Forward:              &defaultLogForward,
		ToStdout:             &defaultLogToStdout,
		SmartLevelEntryLimit: &DefaultSmartVerboseModeEntryLimit,
		IncludeFilters:       make(map[string][]interface{}),
		ExcludeFilters:       make(map[string][]interface{}),
	}
}

func (lc *LogConfig) AttachDefaultFilters() {
	if lc.ExcludeFilters == nil {
		lc.ExcludeFilters = make(map[string][]interface{})
	}

	// Exclude by default supervisor and feature traces.
	lc.ExcludeFilters[TracesFieldName] = append(lc.ExcludeFilters[TracesFieldName], SupervisorTrace, FeatureTrace, ProcessTrace)
}

// HasIncludeFilter returns true if key-value pair are included in the filtering configuration.
func (lc *LogConfig) HasIncludeFilter(key, value string) bool {
	if _, hasWildcardKey := lc.IncludeFilters[LogFilterWildcard]; hasWildcardKey {
		return true
	}

	values, found := lc.IncludeFilters[key]
	if !found {
		return false
	}

	for _, element := range values {
		if element == value || element == LogFilterWildcard {
			return true
		}
	}
	return false
}

// LogRotateConfig map all log rotator configuration options
type LogRotateConfig struct {
	MaxSizeMb          *int   `yaml:"max_size_mb" envconfig:"max_size_mb"`
	MaxFiles           int    `yaml:"max_files" envconfig:"max_files"`
	CompressionEnabled bool   `yaml:"compression_enabled" envconfig:"compression_enabled"`
	FilePattern        string `yaml:"file_pattern" envconfig:"file_pattern"`
}

func (l *LogRotateConfig) IsSet() bool {
	return l.MaxSizeMb != nil
}

// IsEnabled checks if log rotation is enabled.
func (l *LogRotateConfig) IsEnabled() bool {
	return l.IsSet() && *l.MaxSizeMb > 0
}

// VerboseEnabled return 1 if debug or higher log level is enabled.
// The primary purpose is for backwards compatibility with Verbose int attribute.
func (lc *LogConfig) VerboseEnabled() int {
	if lc.Level == LogLevelDebug || lc.Level == LogLevelTrace {
		return 1
	}

	return 0
}

// NtpConfig map all ntp configuration options.
type NtpConfig struct {
	Pool     []string `yaml:"pool" envconfig:"pool"`
	Enabled  bool     `yaml:"enabled" envconfig:"enabled"`
	Interval uint     `yaml:"interval" envconfig:"interval"`
	Timeout  uint     `yaml:"timeout" envconfig:"timeout"`
}

func NewNtpConfig() NtpConfig {
	return NtpConfig{
		Pool:     defaultNtpPool,
		Enabled:  defaultNtpEnabled,
		Interval: defaultNtpInterval,
		Timeout:  defaultNtpTimeout,
	}
}

func coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func coalesceBool(values ...*bool) bool {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return false
}

func (cfg *Config) Provide() *Config {

	if cfg.dynamicConfig != nil {
		refreshedConfig := cfg.dynamicConfig.Provide()
		if refreshedConfig != nil {
			return refreshedConfig
		}
	}

	return cfg
}

func (config *Config) loadLogConfig() {
	// Add default ExcludeFilters
	defer config.Log.AttachDefaultFilters()

	// populate LogConfig object from old configuration options
	if reflect.DeepEqual(config.Log, LogConfig{}) {
		config.populateLogConfig()
		return
	}

	// if windows and config.Log.Rotate == nil -> fill defaults for windows
	if !config.Log.Rotate.IsSet() {
		config.Log.Rotate = loadDefaultLogRotation()
	}

	// backwards compatability with non struct log configuration options
	config.LogFile = coalesce(config.Log.File, config.LogFile)
	config.LogFormat = coalesce(config.Log.Format, config.LogFormat)
	config.LogToStdout = coalesceBool(config.Log.ToStdout, &config.LogToStdout)
	if config.Log.SmartLevelEntryLimit != nil {
		config.SmartVerboseModeEntryLimit = *config.Log.SmartLevelEntryLimit
	} else {
		config.Log.SmartLevelEntryLimit = &DefaultSmartVerboseModeEntryLimit
	}
	switch config.Log.Level {
	case LogLevelSmart:
		config.Verbose = SmartVerboseLogging
	case LogLevelWarn, LogLevelInfo, LogLevelError:
		config.Verbose = 0
	case LogLevelDebug:
		config.Verbose = VerboseLogging
	case LogLevelTrace:
		config.Verbose = TraceLogging
	}
	if config.Log.Forward != nil && *config.Log.Forward {
		switch config.Log.Level {
		case LogLevelDebug:
			config.Verbose = TroubleshootLogging
		case LogLevelTrace:
			config.Verbose = TraceTroubleshootLogging
		}
	}
}

func (config *Config) populateLogConfig() {
	boolPtr := func(a bool) *bool {
		return &a
	}
	config.Log = LogConfig{
		File:                 config.LogFile,
		Format:               config.LogFormat,
		ToStdout:             &config.LogToStdout,
		Forward:              boolPtr(false),
		SmartLevelEntryLimit: &config.SmartVerboseModeEntryLimit,
	}

	switch config.Verbose {
	case SmartVerboseLogging:
		config.Log.Level = LogLevelSmart
	case NonVerboseLogging:
		config.Log.Level = LogLevelInfo
	case VerboseLogging, TroubleshootLogging:
		config.Log.Level = LogLevelDebug
	case TraceLogging, TraceTroubleshootLogging:
		config.Log.Level = LogLevelTrace
	// set log level to trace for any value > 5
	default:
		config.Log.Level = LogLevelTrace
	}
	if config.Verbose == TroubleshootLogging || config.Verbose == TraceTroubleshootLogging {
		config.Log.Forward = boolPtr(true)
	}
}

// LogForward log forwarder config values.
type LogForward struct {
	Troubleshoot Troubleshoot
	ConfigsDir   string
	HomeDir      string
	License      string
	IsFedramp    bool
	IsStaging    bool
	ProxyCfg     LogForwardProxy
	RetryLimit   string
}

type LogForwardProxy struct {
	IgnoreSystemProxy bool
	Proxy             string
	CABundleFile      string
	CABundleDir       string
	ValidateCerts     bool
}

// NewLogForward creates a valid log forwarder config.
func NewLogForward(config *Config, troubleshoot Troubleshoot) LogForward {
	return LogForward{
		Troubleshoot: troubleshoot,
		ConfigsDir:   config.LoggingConfigsDir,
		HomeDir:      config.LoggingHomeDir,
		License:      config.License,
		IsFedramp:    config.Fedramp,
		IsStaging:    config.Staging,
		RetryLimit:   config.LoggingRetryLimit,
		ProxyCfg: LogForwardProxy{
			IgnoreSystemProxy: config.IgnoreSystemProxy,
			Proxy:             config.Proxy,
			CABundleFile:      config.CABundleFile,
			CABundleDir:       config.CABundleDir,
			ValidateCerts:     config.ProxyValidateCerts,
		},
	}
}

// IsTroubleshootMode triggers FluentBit log forwarder to submit agent log. If agent is not running
// under systemd service this mode enables agent logging to a log file (if not present already).
func (lc *LogConfig) IsTroubleshootMode() bool {
	if lc.Forward != nil {
		return *lc.Forward
	}

	return false
}

// IsSmartLogging returns if smart level is enabled.
func (lc *LogConfig) IsSmartLogging() bool {
	return lc.Level == LogLevelSmart
}

// IsStdoutEnabled returns if logs should be logged to stdout.
func (lc *LogConfig) IsStdoutEnabled() bool {
	if lc.ToStdout != nil {
		return *lc.ToStdout
	}

	return defaultLogToStdout
}

// GetSmartLogLevelLimit returns the defined smart log level limit.
func (lc *LogConfig) GetSmartLogLevelLimit() int {
	if lc.SmartLevelEntryLimit != nil {
		return *lc.SmartLevelEntryLimit
	}

	return DefaultSmartVerboseModeEntryLimit
}

// GetDefaultLogFile sets log file to defined app data dir or default.
func (c *Config) GetDefaultLogFile() string {
	if c.AppDataDir == "" {
		return defaultLogFile
	}
	return filepath.Join(c.AppDataDir, "newrelic-infra.log")
}

// GetLogFile provides configured log file.
func (c *Config) GetLogFile() string {
	if c.Log.File == "" || c.Log.File == "true" {
		return c.GetDefaultLogFile()
	}

	return c.Log.File
}

// LogInfo will log the configuration.
// It obfuscates sensitive information and hide private configs.
func (c *Config) LogInfo() {
	configFields, err := c.PublicFields()
	if err != nil {
		clog.WithError(err).Error("failed to log config")
		return
	}

	clog.WithFieldsF(func() logrus.Fields {
		f := logrus.Fields{}
		for key, value := range configFields {
			f[key] = value
		}
		return f
	}).Debug("Loaded configuration.")
}

func (c *Config) SetBoolValueByYamlAttribute(attribute string, value bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	s := reflect.ValueOf(c)
	s = s.Elem()
	t := s.Type()
	for i := 0; i < s.NumField(); i++ {
		ftype := t.Field(i)
		ftags := ftype.Tag
		if ftags.Get("yaml") == attribute {
			f := s.Field(i)
			f.SetBool(value)
			return nil
		}
	}

	return fmt.Errorf("unknown field for yaml attribute '%s'", attribute)
}

func (c *Config) SetIntValueByYamlAttribute(attribute string, value int64) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	s := reflect.ValueOf(c)
	s = s.Elem()
	t := s.Type()
	for i := 0; i < s.NumField(); i++ {
		ftype := t.Field(i)
		ftags := ftype.Tag
		if ftags.Get("yaml") == attribute {
			f := s.Field(i)
			f.SetInt(value)
			return nil
		}
	}

	return fmt.Errorf("unknown field for yaml attribute '%s'", attribute)
}

// PublicFields returns public config fields values indexed by YAML name. It obfuscates sensitive info.
func (c *Config) PublicFields() (map[string]string, error) {
	valueOfC := reflect.ValueOf(c)

	if valueOfC.Kind() != reflect.Ptr && valueOfC.Kind() != reflect.Interface {
		return nil, errors.New("no interface or pointer")
	}

	valueOfC = valueOfC.Elem()

	if valueOfC.Kind() != reflect.Struct {
		return nil, errors.New("config is not struct")
	}

	typeOfC := valueOfC.Type()

	result := make(map[string]string)
	for i := 0; i < valueOfC.NumField(); i++ {

		fieldValue := valueOfC.Field(i)
		fieldType := typeOfC.Field(i)

		fieldTags := fieldType.Tag

		osName := fieldTags.Get("os")

		if osName != "" && !strings.Contains(osName, runtime.GOOS) {
			continue
		}

		if !fieldValue.CanInterface() || fieldTags.Get("ignored") == "true" {
			continue
		}

		configOption := fieldTags.Get("yaml")
		if configOption == "" {
			continue
		}

		public := fieldTags.Get("public")
		if public == "false" {
			continue
		}

		value := fmt.Sprintf("%v", fieldValue.Interface())
		if public == "obfuscate" {
			value = helpers.HiddenField
		}

		if fieldValue.Kind() == reflect.Ptr {
			value = fmt.Sprintf("%v", fieldValue.Elem())
		}

		result[configOption] = value
	}
	return result, nil
}

func LoadConfig(configFile string) (*Config, error) {
	var filesToCheck []string
	if configFile != "" {
		filesToCheck = append(filesToCheck, configFile)
	}

	filesToCheck = append(filesToCheck, defaultConfigFiles...)
	cfg := NewConfig()
	cfgMetadata, err := config_loader.LoadYamlConfig(cfg, filesToCheck...)
	if err != nil {
		err = fmt.Errorf("%w, %s: %s", ErrUnableToParseConfigFile, configFile, err.Error())
		return cfg, err
	}

	if !cfg.Databind.IsEmpty() {
		databindSources, errD := cfg.Databind.DataSources()
		if errD != nil {
			//nolint:wrapcheck
			return cfg, fmt.Errorf("%w: %v", ErrDatabindApply, errD.Error())
		}
		templateConfig := NewConfig()

		_, errD = config_loader.LoadYamlConfig(templateConfig, filesToCheck...)
		if errD != nil {
			//nolint:wrapcheck
			return cfg, fmt.Errorf("%w: %v", ErrDatabindApply, errD.Error())
		}

		dynamicConfig := DynamicConfig{
			databindSources:        databindSources,
			templateConfigMetadata: cfgMetadata,
			templateConfig:         templateConfig,
			refreshedConfig:        nil,
		}
		cfg.dynamicConfig = &dynamicConfig

		cfg, errD = applyDatabind(&dynamicConfig)
		if errD != nil {
			//nolint:wrapcheck
			return cfg, fmt.Errorf("%w: %v", ErrDatabindApply, errD.Error())
		}
	}

	// After the config file has loaded, override via any environment variables
	configOverride(cfg)

	cfg.RunMode, cfg.AgentUser, cfg.ExecutablePath = runtimeValues()

	// Move any other post processing steps that clean up or announce settings to be
	// after both config file and env variable processing is complete. Need to review each of the items
	// above and place each one at the bottom of this ordering
	err = NormalizeConfig(cfg, *cfgMetadata)

	return cfg, err
}

func applyDatabind(dynamicConfig *DynamicConfig) (*Config, error) {
	var err error

	if dynamicConfig == nil {
		//nolint:wrapcheck
		return nil, ErrNoDatabindFound
	}

	vals, err := databind.Fetch(dynamicConfig.databindSources)
	if err != nil {
		return nil, err
	}

	if vals.VarsLen() == 0 {
		//nolint:wrapcheck
		return nil, ErrNoDatabindSources
	}

	matches, errD := databind.Replace(&vals, dynamicConfig.templateConfig)
	if errD != nil {
		return nil, err
	}

	if len(matches) != 1 {
		//nolint:wrapcheck
		return nil, ErrUnexpectedVariablesAmount
	}
	transformed := matches[0]

	resultConfig, ok := transformed.Variables.(*Config)
	if !ok {
		//nolint:wrapcheck
		return nil, fmt.Errorf("%w", ErrUnexpectedVariablesType)
	}

	resultConfig.dynamicConfig = dynamicConfig

	return resultConfig, nil
}

// NewConfig returns the default Config.
func NewConfig() *Config {
	return &Config{
		lock: sync.Mutex{},
		// The following values are not configurable by the user
		ConnectEnabled:                defaultConnectEnabled,
		FirstReapInterval:             defaultFirstReapInterval,
		ReapInterval:                  defaultReapInterval,
		SendInterval:                  defaultSendInterval,
		PidFile:                       defaultPidFile,
		InventoryIngestEndpoint:       defaultInventoryIngestEndpoint,
		MetricsIngestEndpoint:         defaultMetricsIngestEndpoint,
		DMIngestEndpoint:              defaultDMIngestEndpoint,
		IdentityIngestEndpoint:        defaultIdentityIngestEndpoint,
		CommandChannelEndpoint:        defaultCmdChannelEndpoint,
		CommandChannelIntervalSec:     defaultCmdChannelIntervalSec,
		AgentDir:                      defaultAgentDir,
		ConfigDir:                     defaultConfigDir,
		SupervisorRpcSocket:           defaultSupervisorRpcSock,
		DebugLogSec:                   defaultDebugLogSec,
		TruncTextValues:               defaultTruncTextValues,
		LogFormat:                     defaultLogFormat,
		LoggingRetryLimit:             defaultLoggingRetryLimit,
		HTTPServerHost:                defaultHTTPServerHost,
		HTTPServerPort:                defaultHTTPServerPort,
		TCPServerPort:                 defaultTCPServerPort,
		StatusServerPort:              defaultStatusServerPort,
		DockerApiVersion:              DefaultDockerApiVersion,
		FingerprintUpdateFreqSec:      defaultFingerprintUpdateFreqSec,
		CloudMetadataExpiryInSec:      defaultCloudMetadataExpiryInSec,
		RegisterConcurrency:           defaultRegisterConcurrency,
		RegisterBatchSize:             defaultRegisterBatchSize,
		RegisterFrequencySecs:         defaultRegisterFrequencySecs,
		HeartBeatSampleRate:           DefaultHeartBeatFrequencySecs,
		DMSubmissionPeriod:            DefaultDMPeriodSecs,
		ProxyConfigPlugin:             defaultProxyConfigPlugin,
		ProxyValidateCerts:            defaultProxyValidateCerts,
		CloudProvider:                 defaultCloudProvider,
		CloudRetryBackOffSec:          defaultCloudRetryBackOffSec,
		CloudMaxRetryCount:            defaultCloudMaxRetryCount,
		CloudMetadataDisableKeepAlive: defaultCloudMetadataDisableKeepAlive,
		RegisterMaxRetryBoSecs:        defaultRegisterMaxRetryBoSecs,
		IgnoreReclaimable:             defaultIgnoreReclaimable,
		DnsHostnameResolution:         defaultDnsHostnameResolution,
		MaxProcs:                      defaultMaxProcs,
		// At the moment, this is an option that would allow us to rollback to the previous behaviour in case of errors
		DisableInventorySplit:       defaultDisableInventorySplit,
		MaxInventorySize:            defaultMaxInventorySize,
		MaxMetricsBatchSizeBytes:    DefaultMaxMetricsBatchSizeBytes,
		MaxMetricBatchEntitiesCount: DefaultMaxMetricBatchEntitiesCount,
		MaxMetricBatchEntitiesQueue: DefaultMaxMetricBatchEntitiesQueue,
		StartupConnectionRetries:    defaultStartupConnectionRetries,
		DisableZeroRSSFilter:        defaultDisableZeroRSSFilter,
		DisableWinSharedWMI:         defaultDisableWinSharedWMI,
		InventoryArchiveEnabled:     defaultInventoryArchiveEnabled,
		CompactEnabled:              defaultCompactEnabled,
		StripCommandLine:            DefaultStripCommandLine,
		NetworkInterfaceFilters:     defaultNetworkInterfaceFilters,
		SelinuxEnableSemodule:       defaultSelinuxEnableSemodule,
		OfflineTimeToReset:          DefaultOfflineTimeToReset,
		FilesConfigOn:               defaultFilesConfigOn,
		PayloadCompressionLevel:     defaultPayloadCompressionLevel,
		EnableWinUpdatePlugin:       defaultWinUpdatePlugin,
		LogToStdout:                 defaultLogToStdout,
		IpData:                      defaultIpData,
		ContainerMetadataCacheLimit: DefaultContainerCacheMetadataLimit,
		PartitionsTTL:               defaultPartitionsTTL,
		StartupConnectionTimeout:    defaultStartupConnectionTimeout,
		MetricsNFSSampleRate:        DefaultMetricsNFSSampleRate,
		SmartVerboseModeEntryLimit:  DefaultSmartVerboseModeEntryLimit,
		DefaultIntegrationsTempDir:  defaultIntegrationsTempDir,
		IncludeMetricsMatchers:      defaultMetricsMatcherConfig,
		InventoryQueueLen:           DefaultInventoryQueue,
		NtpMetrics:                  NewNtpConfig(),
		Http:                        NewHttpConfig(),
		AgentTempDir:                defaultAgentTempDir,
	}
}

// NewTest creates a default testing Config.
func NewTest(dataDir string) *Config {
	c := NewConfig()
	c.AgentDir = dataDir
	c.AppDataDir = dataDir
	c.OfflineLoggingMode = true

	return c
}

// NewTestWithDeltas creates a default testing Config submitting deltas.
func NewTestWithDeltas(dataDir string) *Config {
	c := NewTest(dataDir)
	c.OfflineLoggingMode = false

	return c
}

// GenerateInventoryURL will use the agent configuration to generate the url required for inventory endpoint.
func (c Config) GenerateInventoryURL() string {
	inventoryURL := fmt.Sprintf("%s/%s", c.CollectorURL,
		strings.TrimPrefix(c.InventoryIngestEndpoint, "/"))
	if os.Getenv("DEV_INVENTORY_INGEST_URL") != "" {
		inventoryURL = os.Getenv("DEV_INVENTORY_INGEST_URL")
	}
	return strings.TrimSuffix(inventoryURL, "/")
}

func (c *Config) DMIngestURL() string {
	return fmt.Sprintf("%s%s", c.MetricURL, c.DMIngestEndpoint)
}

func isConfigDefined(key string, cfgMetadata config_loader.YAMLMetadata) bool {
	prefixedKey := strings.ToUpper(fmt.Sprint(envPrefix, "_", key))
	if os.Getenv(prefixedKey) != "" {
		return true
	}
	// despite we don't use env vars without the NRIA_ prefix, the `configOverride` function is also
	// handling the environment variables without the prefix. We consider here this case for consistency
	upperKey := strings.ToUpper(key)
	if os.Getenv(upperKey) != "" {
		return true
	}
	if cfgMetadata == nil {
		return false
	}
	return cfgMetadata.Contains(key)
}

// ValidateConfigFrequencySetting returns a valid sampling frequency
// according to the following rules:
//
// disable && req == FREQ_DEFAULT_SAMPLING -> FREQ_DISABLE_SAMPLING
// req <= FREQ_DISABLE_SAMPLING -> FREQ_DISABLE_SAMPLING
// req >= FREQ_DEFAULT_SAMPLING & req < min -> Provided default value `def`
// req > min -> Requested value `req`
//
// Plugins have to implement the logic in which they disable themselves
// if their frequency equals FREQ_DISABLE_SAMPLING.
func ValidateConfigFrequencySetting(req, min, def int64, disable bool) time.Duration {
	if req <= FREQ_DISABLE_SAMPLING || disable && req == FREQ_DEFAULT_SAMPLING {
		return FREQ_DISABLE_SAMPLING
	}

	if req >= FREQ_DEFAULT_SAMPLING && req < min {
		return time.Duration(def)
	}
	return time.Duration(req)
}

func JitterFrequency(freqInSec time.Duration) time.Duration {
	if freqInSec < time.Second {
		return time.Second
	}
	randomSeconds := rand.Int63n(int64(freqInSec / time.Second))
	if randomSeconds < 1 {
		randomSeconds = 1
	}
	return time.Duration(randomSeconds) * time.Second
}

func calculateCollectorURL(licenseKey string, staging, fedramp bool) string {
	if fedramp {
		return defaultSecureFederalURL
	}

	return fmt.Sprintf(baseCollectorURL, urlEnvironmentPrefix(staging), urlRegionPrefix(licenseKey))
}

func calculateIdentityURL(licenseKey string, staging, fedramp bool) string {
	if staging {
		return calculateIdentityStagingURL(licenseKey)
	}
	if fedramp {
		return defaultSecureFedralIdentityURL
	}
	return calculateIdentityProductionURL(licenseKey)
}

func calculateIdentityProductionURL(licenseKey string) string {
	// only EU supported
	if license.IsRegionEU(licenseKey) {
		return defaultIdentityURLEu
	}
	return defaultIdentityURL
}

func calculateIdentityStagingURL(licenseKey string) string {
	// only EU supported
	if license.IsRegionEU(licenseKey) {
		return defaultIdentityStagingURLEu
	}
	return defaultIdentityStagingURL
}

func calculateCmdChannelURL(licenseKey string, staging, fedramp bool) string {
	if staging {
		return calculateCmdChannelStagingURL(licenseKey)
	}
	if fedramp {
		return defaultSecureFedralCmdChannelURL
	}
	return calculateCmdChannelProductionURL(licenseKey)
}

func calculateCmdChannelProductionURL(licenseKey string) string {
	// only EU supported
	if license.IsRegionEU(licenseKey) {
		return defaultCmdChannelURLEu
	}
	return defaultCmdChannelURL
}

func calculateCmdChannelStagingURL(licenseKey string) string {
	// only EU supported
	if license.IsRegionEU(licenseKey) {
		return defaultCmdChannelStagingURLEu
	}
	return defaultCmdChannelStagingURL
}

func calculateSelfInstrumentationApmHost(licenseKey string, staging bool, fedramp bool) string {
	if staging {
		return defaultAPMCollectorHostStaging
	}
	if fedramp {
		return defaultSecureFederalAPMCollectorHost
	}
	return calculateSelfInstrumentationApmProductionHost(licenseKey)
}

func calculateSelfInstrumentationApmProductionHost(licenseKey string) string {
	// only EU supported
	if license.IsRegionEU(licenseKey) {
		return defaultAPMCollectorHostEu
	}
	return defaultAPMCollectorHost
}

func calculateDimensionalMetricURL(collectorURL string, licenseKey string, staging, fedramp bool) string {
	if collectorURL != "" {
		return collectorURL
	}

	if fedramp && !staging {
		return defaultSecureFederalMetricURL
	}

	return fmt.Sprintf(baseDimensionalMetricURL, urlEnvironmentPrefix(staging), urlRegionPrefix(licenseKey))
}

func urlEnvironmentPrefix(staging bool) string {
	if staging {
		return "staging-"
	}
	return ""
}

func urlRegionPrefix(licenseKey string) string {
	if license.IsRegionEU(licenseKey) {
		return "eu."
	}

	return ""
}

func NormalizeConfig(cfg *Config, cfgMetadata config_loader.YAMLMetadata) (err error) {
	nlog := clog.WithField("action", "NormalizeConfig")

	cfg.IgnoredInventoryPathsMap = make(map[string]struct{})
	for _, p := range cfg.IgnoredInventoryPaths {
		cfg.IgnoredInventoryPathsMap[strings.ToLower(p)] = struct{}{}
	}

	if cfg.Features == nil {
		cfg.Features = make(map[string]bool)
	}

	// Setting default values
	if cfg.License == "" {
		err = fmt.Errorf("no license key, please add it to agent's config file or NRIA_LICENSE_KEY environment variable")
		return
	}

	cfg.License = strings.TrimSpace(cfg.License)
	if !license.IsValid(cfg.License) {
		err = fmt.Errorf("invalid license: %s, check agent's config file or NRIA_LICENSE_KEY environment variable", cfg.License)
		return
	}

	//  Map new Log configuration
	cfg.loadLogConfig()

	// set corresponding log level
	logLevel, err := log.ParseLevel(cfg.Log.Level)
	if err != nil {
		logLevel = logrus.InfoLevel
		nlog.WithError(err).
			Warn("couldn't parse log level, info log level will be set")
	}

	log.SetLevel(logLevel)
	logrus.SetLevel(logLevel)

	// dm URL is calculated based on collector url, it should be set before get it default value
	cfg.MetricURL = calculateDimensionalMetricURL(cfg.CollectorURL, cfg.License, cfg.Staging, cfg.Fedramp)

	if cfg.CollectorURL == "" {
		cfg.CollectorURL = calculateCollectorURL(cfg.License, cfg.Staging, cfg.Fedramp)
	}

	nlog.WithField("collectorURL", cfg.CollectorURL).Debug("Collector URL")

	if cfg.IdentityURL == "" {
		cfg.IdentityURL = calculateIdentityURL(cfg.License, cfg.Staging, cfg.Fedramp)
	}

	if cfg.CommandChannelURL == "" {
		cfg.CommandChannelURL = calculateCmdChannelURL(cfg.License, cfg.Staging, cfg.Fedramp)
	}

	// InventoryIngestEndpoint default value defined in NewConfig
	nlog.WithField("InventoryIngestEndpoint", cfg.InventoryIngestEndpoint).
		Debug("Inventory ingest endpoint.")

	if cfg.ConnectEnabled {
		cfg.MetricsIngestEndpoint = defaultMetricsIngestV2Endpoint
	}

	if len(cfg.StatusEndpoints) == 0 {
		// https://docs.newrelic.com/docs/using-new-relic/cross-product-functions/install-configure/networks/#infrastructure
		cfg.StatusEndpoints = []string{
			cfg.IdentityURL + cfg.IdentityIngestEndpoint,
			cfg.CommandChannelURL + cfg.CommandChannelEndpoint,
			cfg.CollectorURL + cfg.MetricsIngestEndpoint,
			cfg.CollectorURL + cfg.InventoryIngestEndpoint,
			// cfg.DMIngestURL(), // dimensional metrics without shimming not available yet
			// no endpoint value to checking log ingest reachability
		}
	}

	// MetricsIngestEndpoint default value defined in NewConfig
	nlog.WithField("MetricsIngestEndpoint", cfg.MetricsIngestEndpoint).
		Debug("Metrics ingest endpoint.")

	// IdentityIngestEndpoint default value defined in NewConfig
	nlog.WithField("IdentityIngestEndpoint", cfg.IdentityIngestEndpoint).
		Debug("Identity ingest endpoint.")

	// Remove leading slashes - everything which posts data should add a path starting with /
	strings.TrimSuffix(cfg.CollectorURL, "/")

	// This environment variable is for internal use only
	if os.Getenv("INFRASTRUCTURE_OFFLINE_MODE") == "1" {
		nlog.Debug("Offline Logging mode enabled.")
		cfg.OfflineLoggingMode = true
	}

	// AgentDir default value defined in NewConfig
	nlog.WithField("AgentDir", cfg.AgentDir).Debug("Default output directory.")

	if defaultAppDataDir != "" && cfg.AppDataDir == "" {
		cfg.AppDataDir = defaultAppDataDir
		nlog.WithField("AppDataDir", cfg.AppDataDir).Debug("Application data directory.")
	}

	if cfg.AppDataDir != "" {
		if err = disk.MkdirAll(cfg.AppDataDir, 0o755); err != nil {
			nlog.WithError(err).Warn("can't create application data directory. Setting it to default")
			cfg.AppDataDir = ""
		}
	}

	if cfg.LoggingConfigsDir == "" {
		cfg.LoggingConfigsDir = filepath.Join(cfg.ConfigDir, defaultLoggingConfigsDir)
	}

	if cfg.LoggingBinDir == "" {
		if runtime.GOOS == "linux" {
			cfg.LoggingBinDir = defaultLoggingBinDir
		} else if runtime.GOOS == "windows" {
			cfg.LoggingBinDir = filepath.Join(cfg.AgentDir, DefaultIntegrationsDir, defaultLoggingBinDir)
		}
	}

	if cfg.FluentBitExePath == "" {
		cfg.FluentBitExePath = filepath.Join(cfg.LoggingBinDir, defaultFluentBitExe)
	}

	if cfg.LoggingHomeDir == "" {
		cfg.LoggingHomeDir = filepath.Join(cfg.AgentDir, DefaultIntegrationsDir, defaultLoggingHomeDir)
	}

	if cfg.FluentBitParsersPath == "" {
		cfg.FluentBitParsersPath = filepath.Join(cfg.LoggingHomeDir, defaultFluentBitParsers)
	}

	if cfg.FluentBitNRLibPath == "" {
		cfg.FluentBitNRLibPath = filepath.Join(cfg.LoggingHomeDir, defaultFluentBitNRLib)
	}

	cfg.PluginInstanceDirs = helpers.RemoveEmptyAndDuplicateEntries(
		[]string{cfg.PluginDir, defaultPluginInstanceDir, filepath.Join(cfg.AgentDir, defaultPluginActiveConfigsDir)})

	if cfg.Log.File == "" && runtime.GOOS == "windows" {
		cfg.Log.File = "true"
	}

	if cfg.Log.File == "true" {
		cfg.Log.File = cfg.GetDefaultLogFile()
		nlog.WithField("LogFile", cfg.Log.File).Debug("Logging to file.")
	}

	// Caution: PluginConfigFiles is ALWAYS defined with the default value. Is this right? Be aware any change could affect backwards compatibilities.
	cfg.PluginConfigFiles = defaultPluginConfigFiles

	if cfg.PayloadCompressionLevel < gzip.NoCompression || cfg.PayloadCompressionLevel > gzip.BestCompression {
		nlog.WithFields(logrus.Fields{
			"provided": cfg.PayloadCompressionLevel,
			"default":  defaultPayloadCompressionLevel,
		}).Warn("Compression Level set is invalid, overriding it to the default payload compression level")
		cfg.PayloadCompressionLevel = defaultPayloadCompressionLevel
	}
	nlog.WithField("PayloadCompressionLevel", cfg.PayloadCompressionLevel).Debug("Payload Compression Level.")

	nlog.WithField("CompactEnabled", cfg.CompactEnabled).Debug("Repository compaction.")

	if cfg.CompactThreshold == 0 {
		cfg.CompactThreshold = uint64(defaultCompactThreshold)
	} else {
		cfg.CompactThreshold = cfg.CompactThreshold * 1024 * 1024
	}

	if cfg.MetricsSystemSampleRate < FREQ_INTERVAL_FLOOR_SYSTEM_METRICS && cfg.MetricsSystemSampleRate > FREQ_DISABLE_SAMPLING {
		cfg.MetricsSystemSampleRate = FREQ_INTERVAL_FLOOR_SYSTEM_METRICS
	}
	nlog.WithField("MetricsSystemSampleRate", cfg.MetricsSystemSampleRate).Debug("Metrics System Sample Rate.")

	if cfg.MetricsStorageSampleRate < FREQ_INTERVAL_FLOOR_STORAGE_METRICS && cfg.MetricsStorageSampleRate > FREQ_DISABLE_SAMPLING {
		cfg.MetricsStorageSampleRate = DefaultStorageSamplerRateSecs
	}
	nlog.WithField("MetricsStorageSampleRate", cfg.MetricsStorageSampleRate).Debug("Metrics Storage Sample Rate.")

	if cfg.MetricsNetworkSampleRate < FREQ_INTERVAL_FLOOR_STORAGE_METRICS && cfg.MetricsNetworkSampleRate > FREQ_DISABLE_SAMPLING {
		cfg.MetricsNetworkSampleRate = FREQ_INTERVAL_FLOOR_STORAGE_METRICS
	}
	nlog.WithField("MetricsNetworkSampleRate", cfg.MetricsNetworkSampleRate).Debug("Metrics Network Sample Rate.")

	if cfg.MetricsProcessSampleRate < FREQ_INTERVAL_FLOOR_PROCESS_METRICS && cfg.MetricsProcessSampleRate > FREQ_DISABLE_SAMPLING {
		cfg.MetricsProcessSampleRate = FREQ_INTERVAL_FLOOR_PROCESS_METRICS
	}
	nlog.WithField("MetricsProcessSampleRate", cfg.MetricsProcessSampleRate).Debug("Metrics Process Sample Rate.")

	nlog.WithField("FilesConfigOn", cfg.FilesConfigOn).Debug("Configuration file monitoring.")

	if cfg.NetworkInterfaceFilters == nil || len(cfg.NetworkInterfaceFilters) == 0 {
		nlog.Info("No Network Interface Filters are defined. The agent will monitor all detectable and supported interfaces.")
	}

	for filter, nica := range cfg.NetworkInterfaceFilters {
		for _, nic := range nica {
			nlog.WithFields(logrus.Fields{"interface": nic, "filter": filter}).
				Debug("Using Network Interface Filter Set.")
		}
	}

	for _, env := range defaultPassthroughEnvironment {
		cfg.PassthroughEnvironment = append(cfg.PassthroughEnvironment, env)
	}

	if cfg.RemoveEntitiesPeriod != "" {
		nlog.WithField("RemoveEntitiesPeriod", cfg.RemoveEntitiesPeriod).Debug("Period for removing non-reporting entities.")
	}

	if _, err := time.ParseDuration(cfg.StartupConnectionTimeout); err != nil {
		nlog.WithFields(logrus.Fields{
			"provided": cfg.StartupConnectionTimeout,
			"default":  defaultStartupConnectionTimeout,
		}).Warn("wrong format for 'startup_connection_timeout' property. Assuming default")
		cfg.StartupConnectionTimeout = defaultStartupConnectionTimeout
	}

	if cfg.MaxMetricsBatchSizeBytes > DefaultMaxMetricsBatchSizeBytes || cfg.MaxMetricsBatchSizeBytes <= 0 {
		cfg.MaxMetricsBatchSizeBytes = DefaultMaxMetricsBatchSizeBytes
	}

	if cfg.MaxMetricBatchEntitiesCount > DefaultMaxMetricBatchEntitiesCount || cfg.MaxMetricBatchEntitiesCount <= 0 {
		cfg.MaxMetricBatchEntitiesCount = DefaultMaxMetricBatchEntitiesCount
	}

	if cfg.MaxMetricBatchEntitiesQueue > DefaultMaxMetricBatchEntitiesQueue || cfg.MaxMetricBatchEntitiesQueue <= 0 {
		cfg.MaxMetricBatchEntitiesQueue = DefaultMaxMetricBatchEntitiesQueue
	}

	// Avoid clients de-facto disabling inventory splitting when we remove the disable_inventory_split function
	if cfg.MaxInventorySize > defaultMaxInventorySize {
		cfg.MaxInventorySize = defaultMaxInventorySize
	}

	if _, err := time.ParseDuration(cfg.PartitionsTTL); err != nil {
		nlog.WithFields(logrus.Fields{
			"provided": cfg.StartupConnectionTimeout,
			"default":  defaultStartupConnectionTimeout,
		}).Warn("wrong format for 'partitions_ttl' property. Assuming default")
		cfg.PartitionsTTL = defaultPartitionsTTL
	}

	if cfg.FacterHomeDir == "" {
		home, err := getDefaultFacterHomeDir()
		if err != nil {
			nlog.WithError(err).
				Warn("couldn't retrieve the current user's home, the HOME env variable won't be set for running facter")
		}
		cfg.FacterHomeDir = home
	}

	// force WMI sampler on Windows 32-bit
	if cfg.LegacyStorageSampler == false && runtime.GOOS == "windows" && runtime.GOARCH == "386" {
		cfg.LegacyStorageSampler = true
	}

	// DockerApiVersion default value defined in NewConfig
	nlog.WithField("DockerApiVersion", cfg.DockerApiVersion).Debug("Docker client API version.")
	// FingerprintUpdateFreqSec default value defined in NewConfig
	nlog.WithField("FingerprintUpdateFreqSec", cfg.FingerprintUpdateFreqSec).Debug("Fingerprint update freq.")
	// DnsHostnameResolution value defined in NewConfig
	nlog.WithField("DnsHostnameResolution", cfg.DnsHostnameResolution).Debug("DNS hostname resolution.")
	// IgnoreReclaimable value defined in NewConfig
	nlog.WithField("IgnoreReclaimable", cfg.IgnoreReclaimable).Debug("Ignoring reclaimable memory.")

	// CloudType default value if cloud is unknown
	if cfg.CloudProvider != "" && !cloud.Type(cfg.CloudProvider).IsValidCloud() {
		nlog.WithField("CloudType", cfg.CloudProvider).Warn("Invalid cloud provider.")
		cfg.CloudProvider = defaultCloudProvider
	}

	// CloudMaxRetryCount default value defined in NewConfig
	nlog.WithField("CloudMaxRetryCount", cfg.CloudMaxRetryCount).Debug("Cloud detection max retry count on error.")
	// CloudRetryBackOffSec default value defined in NewConfig
	nlog.WithField("CloudRetryBackOffSec", cfg.CloudRetryBackOffSec).Debug("Cloud detection retry backOff on error.")
	// CloudMetadataDisableKeepAlive default value defined in NewConfig
	nlog.WithField("CloudMetadataDisableKeepAlive", cfg.CloudMetadataDisableKeepAlive).Debug("Cloud metadata keep-alive.")
	// ProxyValidateCerts default value defined in NewConfig
	nlog.WithField("ProxyValidateCerts", cfg.ProxyValidateCerts).Debug("Proxy certificate verification.")
	// ProxyConfigPlugin default value defined in NewConfig
	nlog.WithField("ProxyConfigPlugin", cfg.ProxyConfigPlugin).Debug("Default proxy config plugin enabled.")

	if runtime.GOOS == "windows" && !isConfigDefined("win_removable_drives", cfgMetadata) {
		cfg.WinRemovableDrives = defaultWinRemovableDrives
		nlog.WithField("WinRemovableDrives", cfg.WinRemovableDrives).Debug("Using default Windows removable drives storage sampling.")
	}

	// defaultCloudMetadataExpiryInSec default value defined in NewConfig
	nlog.WithField("defaultCloudMetadataExpiryInSec", defaultCloudMetadataExpiryInSec).Debug("Using default cloud metadata expiry time.")

	cfg.IsForwardOnly = cfg.IsForwardOnly || cfg.K8sIntegration

	if cfg.IsForwardOnly {
		cfg.ConnectEnabled = false
	}

	// For backwards compatibility FileDevicesBlacklist is deprecated.
	if len(cfg.FileDevicesBlacklist) > 0 {
		cfg.FileDevicesIgnored = append(cfg.FileDevicesIgnored, cfg.FileDevicesBlacklist...)
	}

	// For backwards compatibility WhitelistProcessSample is deprecated.
	if len(cfg.WhitelistProcessSample) > 0 {
		cfg.AllowedListProcessSample = append(cfg.AllowedListProcessSample, cfg.WhitelistProcessSample...)
	}

	if cfg.MemProfile != "" && cfg.MemProfileInterval == 0 {
		cfg.MemProfileInterval = defaultMemProfileInterval
	}

	if cfg.SelfInstrumentationApmHost == "" {
		cfg.SelfInstrumentationApmHost = calculateSelfInstrumentationApmHost(cfg.License, cfg.Staging, cfg.Fedramp)
	}

	return
}

func (c *CustomAttributeMap) Decode(value string) error {
	data := []byte(value)

	// Clear current Custom Attribute Map
	for k := range *c {
		delete(*c, k)
	}
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	return nil
}

// DataMap returns the CustomAttributeMap as a data.Map
func (c *CustomAttributeMap) DataMap() (d data.Map) {
	d = data.Map{}

	for k, v := range *c {
		if str, isString := v.(string); isString {
			d[k] = str
		}
	}

	return
}

func (i *IncludeMetricsMap) Decode(value string) error {
	data := []byte(value)

	// Clear current Map
	for k := range *i {
		delete(*i, k)
	}

	if err := yaml.Unmarshal(data, i); err != nil {
		return err
	}
	return nil
}

func (i *LogFilters) Decode(value string) error {
	data := []byte(value)

	// Clear current Map
	for k := range *i {
		delete(*i, k)
	}

	if err := yaml.Unmarshal(data, i); err != nil {
		return err
	}
	return nil
}
