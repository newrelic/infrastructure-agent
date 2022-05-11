// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// This fixture integration submits as inventory all the key=value pairs passed as arguments
// It uses a remote entity.

func main() {
	inventory := protocol.InventoryData{}
	for _, pair := range os.Args[1:] {
		kv := strings.Split(pair, "=")
		if len(kv) < 2 {
			_, _ = fmt.Fprint(os.Stderr, "argument must be in form key=value. Got:", pair)
			os.Exit(-1)
		}
		inventory[kv[0]] = kv[1]
	}
	data := protocol.PluginDataV3{
		PluginOutputIdentifier: protocol.PluginOutputIdentifier{
			Name:               "testing_integration",
			RawProtocolVersion: "2",
			IntegrationVersion: "1.2.3",
		},
		DataSets: []protocol.PluginDataSetV3{{
			Cluster: "local-test",
			Service: "test-service",
			PluginDataSet: protocol.PluginDataSet{Entity: entity.Fields{
				Name:         "localtest",
				Type:         "test",
				IDAttributes: []entity.IDAttribute{{"idkey", "idval"}},
			}, Inventory: map[string]protocol.InventoryData{
				"cliargs": inventory,
			}, Metrics: []protocol.MetricData{},
				Events: []protocol.EventData{}},
		}},
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "can't write JSON:", err.Error())
		os.Exit(-1)
	}
	fmt.Println(string(bytes))
}
