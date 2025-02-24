// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/agent/types"

	event2 "github.com/newrelic/infrastructure-agent/pkg/event"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

func BuildInventoryDataSet(
	entryLog log.Entry,
	inventoryData map[string]protocol.InventoryData,
	labels map[string]string,
	integrationUser string,
	pluginName string,
	entityKey string) types.PluginInventoryDataset {
	var inventoryDataSet types.PluginInventoryDataset

	for key, item := range inventoryData {
		item["id"] = key
		err := verifyInventoryData(item)
		if err != nil {
			entryLog.WithError(err).WithIntegration(pluginName).Warn("couldn't normalize inventory content")
		} else {
			inventoryDataSet = append(inventoryDataSet, item)
		}
	}

	for key, value := range labels {
		inventoryDataSet = append(inventoryDataSet, protocol.InventoryData{
			"id":        fmt.Sprintf("labels/%s", key),
			"value":     value,
			"entityKey": entityKey,
		})
	}

	if integrationUser != "" {
		inventoryDataSet = append(inventoryDataSet, protocol.InventoryData{
			"id":        "integrationUser",
			"value":     integrationUser,
			"entityKey": entityKey,
		})
	}

	return inventoryDataSet
}

func NormalizeEvent(
	entryLog log.Entry,
	event protocol.EventData,
	labels map[string]string,
	extraAnnotations map[string]string,
	integrationUser string,
	entityKey string) protocol.EventData {
	_, ok := event[V1_REQUIRED_EVENT_FIELD]
	if !ok {
		entryLog.WithFields(logrus.Fields{
			"payload":      event,
			"missingField": V1_REQUIRED_EVENT_FIELD,
		}).Warn("invalid event format: missing required field")
		return nil
	}

	normalizedEvent := protocol.EventData{
		"eventType": V1_EVENT_EVENT_TYPE,
		"category":  V1_DEFAULT_EVENT_CATEGORY,
	}
	for key, val := range event {
		if !event2.IsReserved(key) {
			normalizedEvent[key] = val
		}
	}
	for key, value := range labels {
		normalizedEvent[fmt.Sprintf("label.%s", key)] = value
	}
	for key, value := range extraAnnotations {
		// Extra annotations can't override current events
		if _, ok = event[key]; !ok {
			normalizedEvent[key] = value
		}
	}
	if integrationUser != "" {
		normalizedEvent["integrationUser"] = integrationUser
	}

	normalizedEvent["entityKey"] = entityKey

	if attrs, ok := event["attributes"]; ok {
		switch t := attrs.(type) {
		default:
		case map[string]interface{}:
			for key, value := range t {
				// To avoid collisions repeated attributes are namespaced.
				if _, ok := normalizedEvent[key]; ok {
					normalizedEvent[fmt.Sprintf("attr.%s", key)] = value
				} else {
					normalizedEvent[key] = value
				}
			}
		}
	}

	// there are integrations that add the hostname so
	//Let's make sure that we do NOT have hostname in the event.
	delete(normalizedEvent, "hostname")

	return normalizedEvent
}

func verifyInventoryData(item protocol.InventoryData) error {
	if item.SortKey() == "" {
		hash, err := getVariantHash(item)
		if err != nil {
			return fmt.Errorf("couldn't produce a variant hash: %v", err)
		}
		item["id"] = hash
	}
	return nil
}
