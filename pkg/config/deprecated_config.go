// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

type DeprecatedConfig struct {
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

	// FileDevicesBlacklist List of storage devices to be ignored by the agent when gathering StorageSamples.
	// Default: Empty
	// Public: No
	// Deprecated: use FileDevicesIgnored instead.
	FileDevicesBlacklist []string `yaml:"file_devices_blacklist" envconfig:"file_devices_blacklist"`

	// LogToStdout By default all logs are displayed in both standard output and a log file. If you want to disable
	// logs in the standard output you can set this configuration option to FALSE.
	// Default: True
	// Public: Yes
	// Deprecated: use Log.ToStdout instead.
	LogToStdout bool `yaml:"log_to_stdout" envconfig:"log_to_stdout"`

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
}
