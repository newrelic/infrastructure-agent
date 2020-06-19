// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux windows
// +build 386 arm mips mipsle

//
// NOTE: These constants are tuned for 32-bit builds of the agent are are generally
// slower than their 64-bit equivalents. While 32-bit is NOT currently a publically
// available release, these values are being tuned to work with target platform of
// a busy Celeron 1.1GHz, 386, 32-bit environment running Windows Embedded POSReady 7 kernel
// first, and possible Linux OSs second.
//

package config

import "time"

const (
	FREQ_INTERVAL_FLOOR_METRICS         = 15 // seconds, absolute fastest any metrics can run
	FREQ_INTERVAL_FLOOR_SYSTEM_METRICS  = 15 // seconds, fastest that metrics can be configured to sample
	FREQ_INTERVAL_FLOOR_STORAGE_METRICS = 15 // seconds
	FREQ_INTERVAL_FLOOR_NETWORK_METRICS = 15 // seconds
	FREQ_INTERVAL_FLOOR_PROCESS_METRICS = 20 // seconds, process time has great impact on our cap planning, ask before changing

	FREQ_METRICS_SEND_INTERVAL    = FREQ_INTERVAL_FLOOR_METRICS // seconds between sending samples for base metrics (System, Process, etc)
	INITIAL_REAP_MAX_WAIT_SECONDS = 60                          // seconds to wait for all plugins to report before reporting data anyway

	// LINUX PLUGINS
	FREQ_PLUGIN_SYSCTL_UPDATES         = 10 //seconds
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
	FREQ_EXTERNAL_USER_DATA      = 30 // seconds between external user data samples (deprecated user json plugin)
	FREQ_PLUGIN_EXTERNAL_PLUGINS = 30 // seconds

	defaultFirstReapInterval = 1 * time.Second  // inventory: reap every second until first successful reap, then switch to DefaultReapInterval
	defaultReapInterval      = 20 * time.Second // seconds, inventory: fire reap trigger every 10 seconds after first successful reap
	defaultSendInterval      = 20 * time.Second // seconds, inventory: fire send trigger every 10 seconds
)
