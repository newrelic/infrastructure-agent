// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build  !go1.13

package instrumentation

func NewOpentelemetryServer() (exporter Exporter, err error) {
	return NewNoopServer(), nil
}
