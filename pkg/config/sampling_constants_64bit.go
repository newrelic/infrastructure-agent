// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build (linux || darwin || windows || freebsd) && (amd64 || arm64 || mips64 || mips64le || ppc64 || ppc64le || s390x)
// +build linux darwin windows freebsd
// +build amd64 arm64 mips64 mips64le ppc64 ppc64le s390x

//
// NOTE: These constants are tuned for 64-bit builds of the agent are are generally
// faster than their 32-bit equivalents.
//

package config

import "time"

const (
	FREQ_INTERVAL_FLOOR_METRICS         = 5  // seconds, absolute fastest any metrics can run
	FREQ_INTERVAL_FLOOR_SYSTEM_METRICS  = 5  // seconds, fastest that metrics can be configured to sample
	FREQ_INTERVAL_FLOOR_STORAGE_METRICS = 5  // seconds
	FREQ_INTERVAL_FLOOR_NETWORK_METRICS = 10 // seconds
	FREQ_INTERVAL_FLOOR_PROCESS_METRICS = 20 // seconds, process time has great impact on our cap planning, ask before changing

	FREQ_METRICS_SEND_INTERVAL    = FREQ_INTERVAL_FLOOR_METRICS // seconds between sending samples for base metrics (System, Process, etc)
	INITIAL_REAP_MAX_WAIT_SECONDS = 60                          // seconds to wait for all plugins to report before reporting data anyway

	// LINUX PLUGINS
	FREQ_PLUGIN_SYSCTL_UPDATES         = 60 //seconds
	FREQ_PLUGIN_KERNEL_MODULES_UPDATES = 10 //seconds
	FREQ_PLUGIN_USERS_UPDATES          = 15 //seconds
	FREQ_PLUGIN_SSHD_CONFIG_UPDATES    = 15 //seconds
	FREQ_PLUGIN_SUPERVISOR_UPDATES     = 15 //seconds
	FREQ_PLUGIN_DAEMONTOOLS_UPDATES    = 15 //seconds
	FREQ_PLUGIN_SYSTEMD_UPDATES        = 30 // seconds
	FREQ_PLUGIN_SYSVINIT_UPDATES       = 30 // seconds
	FREQ_PLUGIN_UPSTART_UPDATES        = 30 // seconds

	FREQ_PLUGIN_FACTER_UPDATES            = 30 // seconds -- facter plugin
	FREQ_PLUGIN_PACKAGE_MGRS_UPDATES      = 30 // seconds -- rpm, deb plugins. RPM watches /var/lib/rpm/.rpm.lock, dpkg: /var/lib/dpkg/lock
	FREQ_PLUGIN_SELINUX_UPDATES           = 30 // seconds
	FREQ_PLUGIN_HOST_ALIASES              = 30 // seconds
	FREQ_PLUGIN_NETWORK_INTERFACE_UPDATES = 60 // seconds
	FREQ_PLUGIN_CLOUD_SECURITY_UPDATES    = 60 // seconds

	// WINDOWS PLUGINS
	FREQ_PLUGIN_WINDOWS_SERVICES = 30 // seconds, 0 == off, 30 == minimum otherwise: inventory: running services
	FREQ_PLUGIN_WINDOWS_UPDATES  = 60 // seconds

	// BOTH
	FREQ_EXTERNAL_USER_DATA      = 10 // seconds between external user data samples (deprecated user json plugin)
	FREQ_PLUGIN_EXTERNAL_PLUGINS = 30 // seconds

	defaultFirstReapInterval = 1 * time.Second  // inventory: reap every second until first successful reap, then switch to DefaultReapInterval
	defaultReapInterval      = 10 * time.Second // inventory: fire reap trigger every 10 seconds after first successful reap
	defaultSendInterval      = 10 * time.Second // inventory: fire send trigger every 10 seconds
)
