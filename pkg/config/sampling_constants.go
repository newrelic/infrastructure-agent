// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Shared sampling constants
//
package config

const (
	FREQ_DISABLE_SAMPLING = -1 // seconds, don't sample
	FREQ_DEFAULT_SAMPLING = 0  // Use the "default" value for a given sampler type

	MAX_BACKOFF          = 120  // seconds, inventory: if we are in backoff mode this the upper bound interval for non-forced backoff
	RATE_LIMITED_BACKOFF = 3600 // seconds, inventory: if we are told to backoff, wait a longer period of time

	FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE = 10 // seconds, used for specific inventory that may change more quickly and isn't too expensive
	FREQ_MINIMUM_INVENTORY_SAMPLE_RATE      = 30 // seconds, this is the default minimum (fastest) inventory sample rate

	// NOTE: These constants are specified in PLUGIN API, DO NOT CHANGE WITHOUT DISCUSSION
	FREQ_MINIMUM_EXTERNAL_PLUGIN_INTERVAL = 15 // seconds
	FREQ_MAXIMUM_EXTERNAL_PLUGIN_INTERVAL = 60 // seconds, frequency at which warning is shown for possible alert problems

	FREQ_PLUGIN_K8S_INTEGRATION_SAMPLES_UPDATES = 30 // seconds

	// DefaultWMINamespace is the Namespace where the WMI queries will be executed
	DefaultWMINamespace = "root/cimv2"
)
