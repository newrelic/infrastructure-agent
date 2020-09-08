// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package trace

// Feature feature to be traced.
type Feature string

// String stringer method
func (f Feature) String() string {
	return string(f)
}

// List of available features that can be traced.
const (
	ATTR           Feature = "attributes" // custom-attributes
	CONN           Feature = "connect"    // fingerprint connect
	HOSTNAME       Feature = "hostname"
	DM_SUBMISSION  Feature = "dm.submission" // dimensional metrics submission
	METRIC_MATCHER Feature = "metric.match"  // match metric by rule
	INVENTORY      Feature = "inventory"
)

// Helper functions to avoid repeating:
// trace.On(trace.FEATURE, ...)

// Attr always traces custom-attributes feature.
func Attr(format string, args ...interface{}) {
	On(func() bool { return true }, ATTR, format, args...)
}

// AttrOn trace custom-attributes feature on given condition.
func AttrOn(cond Condition, format string, args ...interface{}) {
	On(cond, ATTR, format, args...)
}

// Connect always traces connect feature.
func Connect(format string, args ...interface{}) {
	On(func() bool { return true }, CONN, format, args...)
}

// Hostname always traces hostname feature.
func Hostname(format string, args ...interface{}) {
	On(func() bool { return true }, HOSTNAME, format, args...)
}

// Telemetry traces to "audit" (log payloads) on DM telemetry.
func Telemetry(format string, args ...interface{}) {
	On(func() bool { return true }, DM_SUBMISSION, format, args...)
}

// MetricMatch traces to "audit" log metric match rule.
func MetricMatch(format string, args ...interface{}) {
	On(func() bool { return true }, METRIC_MATCHER, format, args...)
}

// Inventory traces to "audit" inventory.
func Inventory(format string, args ...interface{}) {
	On(func() bool { return true }, INVENTORY, format, args...)
}
