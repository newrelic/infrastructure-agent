// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package trace

import "github.com/newrelic/infrastructure-agent/pkg/log"

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
	V3_SUBMISSION  Feature = "v3.submission" // non dimensional metrics (integration protocol v3) submission
	METRIC_MATCHER Feature = "metric.match"  // match metric by rule
	INVENTORY      Feature = "inventory"
	LOG_FWD        Feature = "log.fw"
	CMDREQ         Feature = "cmdreq"  // command requests from integrations
	PROC           Feature = "proc"    // process data
	SAMPLER        Feature = "sampler" // sampler metrics
)

// Helper functions to avoid repeating:
// trace.On(trace.FEATURE, ...)

// Attr always traces custom-attributes feature.
func Attr(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, ATTR, entry, format, args...)
}

// AttrOn trace custom-attributes feature on given condition.
func AttrOn(entry log.Entry, cond Condition, format string, args ...interface{}) {
	On(cond, ATTR, entry, format, args...)
}

// Connect always traces connect feature.
func Connect(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, CONN, entry, format, args...)
}

// Hostname always traces hostname feature.
func Hostname(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, HOSTNAME, entry, format, args...)
}

// NonDMSubmission traces NR platform non-dimensional metrics submission payloads.
func NonDMSubmission(payload []byte) {
	On(func() bool { return true }, V3_SUBMISSION, nil, string(payload))
}

// Telemetry traces to "audit" (log payloads) on DM telemetry.
func Telemetry(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, DM_SUBMISSION, entry, format, args...)
}

// MetricMatch traces to "audit" log metric match rule.
func MetricMatch(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, METRIC_MATCHER, entry, format, args...)
}

// Inventory traces to "audit" inventory.
func Inventory(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, INVENTORY, entry, format, args...)
}

// LogFwdOutput traces to "audit" log-forwarder output.
func LogFwdOutput(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, LOG_FWD, entry, format, args...)
}

// CmdReq traces to "audit" command request payloads.
func CmdReq(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, CMDREQ, entry, format, args...)
}

// Proc traces to "audit" process sampling.
func Proc(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, PROC, entry, format, args...)
}

// Sampler traces to "audit" system samplers (cpu, disk, network, etc) stats.
func Sampler(entry log.Entry, format string, args ...interface{}) {
	On(func() bool { return true }, SAMPLER, entry, format, args...)
}
