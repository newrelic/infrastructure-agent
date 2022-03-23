// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package trace

import "github.com/sirupsen/logrus"

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
	CMDREQ         Feature = "cmdreq" // command requests from integrations
	PROC           Feature = "proc"   // process data
)

// Helper functions to avoid repeating:
// trace.On(trace.FEATURE, ...)

// Attr always traces custom-attributes feature.
func Attr(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, ATTR, fields, format, args...)
}

// AttrOn trace custom-attributes feature on given condition.
func AttrOn(fields logrus.Fields, cond Condition, format string, args ...interface{}) {
	On(cond, ATTR, fields, format, args...)
}

// Connect always traces connect feature.
func Connect(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, CONN, fields, format, args...)
}

// Hostname always traces hostname feature.
func Hostname(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, HOSTNAME, fields, format, args...)
}

// NonDMSubmission traces NR platform non-dimensional metrics submission payloads.
func NonDMSubmission(payload []byte) {
	On(func() bool { return true }, V3_SUBMISSION, nil, string(payload))
}

// Telemetry traces to "audit" (log payloads) on DM telemetry.
func Telemetry(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, DM_SUBMISSION, fields, format, args...)
}

// MetricMatch traces to "audit" log metric match rule.
func MetricMatch(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, METRIC_MATCHER, fields, format, args...)
}

// Inventory traces to "audit" inventory.
func Inventory(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, INVENTORY, fields, format, args...)
}

// LogFwdOutput traces to "audit" log-forwarder output.
func LogFwdOutput(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, LOG_FWD, fields, format, args...)
}

// CmdReq traces to "audit" command request payloads.
func CmdReq(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, CMDREQ, fields, format, args...)
}

// Proc traces to "audit" process sampling.
func Proc(fields logrus.Fields, format string, args ...interface{}) {
	On(func() bool { return true }, PROC, fields, format, args...)
}
