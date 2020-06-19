// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import (
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var (
	ProcessSample = metrics.ProcessSample{
		BaseEvent: sample.BaseEvent{
			EntityKey: "my-entity-key",
		},
		ProcessDisplayName: "foo",
		ProcessID:          13,
		CommandName:        "bar",
		User:               "baz",
		MemoryRSSBytes:     1,
		MemoryVMSBytes:     2,
		CPUPercent:         3,
		CPUUserPercent:     4,
		CPUSystemPercent:   5,
		ContainerImage:     "foo",
		ContainerImageName: "bar",
		ContainerName:      "baz",
		ContainerID:        "qux",
		Contained:          "true",
	}
)
