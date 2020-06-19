// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import "github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"

var (
	ExpectedMetadataDelta = []*inventoryapi.RawDelta{
		{
			Source:   "metadata/host_aliases",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"display_name": map[string]interface{}{
					"alias": "display-name",
					"id":    "display_name",
				},
				"hostname": map[string]interface{}{
					"id":    "hostname",
					"alias": "foobar",
				},
				"hostname_short": map[string]interface{}{
					"id":    "hostname_short",
					"alias": "foo",
				},
			},
		},
		{
			Source:   "metadata/agent_config",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				// Verifying some config we already set in the tests, as well as some
				// fields we just know exist
				"infrastructure": map[string]interface{}{
					"DisplayName":           map[string]interface{}{"value": "display-name"},
					"OverrideHostEtc":       map[string]interface{}{"value": ""},
					"MaxProcs":              AnyValue,
					"IgnoredInventoryPaths": AnyValue,
				},
			},
		},
	}

	ExpectedSysctlDelta = []*inventoryapi.RawDelta{
		{
			Source:   "kernel/sysctl",
			ID:       1,
			FullDiff: true,
			// Checking some common /proc/sys entries that should exist in any linux host
			Diff: map[string]interface{}{
				".fs.file-max":    AnyValue,
				".fs.overflowgid": AnyValue,
				".fs.overflowuid": AnyValue,
				".kernel.acct": map[string]interface{}{
					"id":           ".kernel.acct",
					"sysctl_value": AnyValue,
				},
				".kernel.ctrl-alt-del": AnyValue,
				".kernel.domainname":   AnyValue,
				".kernel.hostname":     AnyValue,
				".kernel.modprobe":     AnyValue,
				".kernel.msgmax":       AnyValue,
				".kernel.msgmnb":       AnyValue,
				".kernel.panic": map[string]interface{}{
					"id":           ".kernel.panic",
					"sysctl_value": AnyValue,
				},
				".kernel.printk": AnyValue,
				".vm.block_dump": map[string]interface{}{
					"id":           ".vm.block_dump",
					"sysctl_value": AnyValue,
				},
				".vm.dirty_background_ratio": AnyValue,
			},
		},
	}
)
