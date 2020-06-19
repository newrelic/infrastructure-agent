// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"fmt"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

const (
	entityRewriteActionReplace = "replace"
)

// Try to match and replace entityName according to EntityRewrite configuration.
func ApplyEntityRewrite(entityName string, entityRewrite []data.EntityRewrite) string {
	result := entityName

	for _, er := range entityRewrite {
		if er.Action == entityRewriteActionReplace {
			result = strings.Replace(result, er.Match, er.ReplaceField, -1)
		}
	}

	return result
}

func BuildInventoryDataSet(
	entryLog log.Entry,
	inventoryData map[string]protocol.InventoryData,
	labels map[string]string,
	integrationUser string,
	pluginName string,
	entityKey string) agent.PluginInventoryDataset {
	var inventoryDataSet agent.PluginInventoryDataset

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
	for _, key := range V1_ACCEPTED_EVENT_ATTRIBUTES {
		if val, ok := event[key]; ok {
			normalizedEvent[key] = val
		}
	}
	for key, value := range labels {
		normalizedEvent[fmt.Sprintf("label.%s", key)] = value
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
	// Ensure all inventory content consists of strings
	for key, value := range item {
		strValue, ok := value.(string)
		if ok {
			item[key] = strValue
		} else {
			item[key] = fmt.Sprintf("%#v", value)
		}
	}

	if item.SortKey() == "" {
		hash, err := getVariantHash(item)
		if err != nil {
			return fmt.Errorf("couldn't produce a variant hash: %v", err)
		}
		item["id"] = hash
	}
	return nil
}
