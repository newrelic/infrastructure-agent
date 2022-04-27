package v3

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/groupcache/lru"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	event2 "github.com/newrelic/infrastructure-agent/pkg/event"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

const (
	maxEntityAttributeCount    = 240 // 254 - 14 (reserved for agent decorations) https://docs.newrelic.com/docs/insights/insights-data-sources/custom-data/insights-custom-data-requirements-limits
	entityMetricsLengthWarnMgs = "metric attributes exceeds 240 limit, some might be lost"

	// These two constants can be found in V4 integrations as well
	labelPrefix     = "label."
	labelPrefixTrim = 6
)

const (
	V1_DEFAULT_PLUGIN_CATEGORY = "integration"
	V1_DEFAULT_EVENT_CATEGORY  = "notifications"
	V1_REQUIRED_EVENT_FIELD    = "summary"
	V1_EVENT_EVENT_TYPE        = "InfrastructureEvent"
)

var (
	rlog   = log.WithComponent("PluginRunner")
	logLRU = lru.New(1000) // avoid flooding the log with violations for the same entity
)

func EmitDataSet(
	ctx agent.AgentContext,
	emitter agent.PluginEmitter,
	pluginName string,
	pluginVersion string,
	integrationUser string,
	dataSet protocol.PluginDataSetV3,
	extraAnnotations map[string]string,
	labels map[string]string,
	entityRewrite []data.EntityRewrite,
	protocolVersion int,
) error {
	elog := rlog.WithField("action", "EmitDataSet")

	agentIdentifier := ctx.EntityKey()

	idLookup := ctx.IDLookup()
	entityKey, err := dataSet.Entity.ResolveUniqueEntityKey(agentIdentifier, idLookup, entityRewrite, protocolVersion)
	if err != nil {
		return fmt.Errorf("couldn't determine a unique entity Key: %s", err.Error())
	}

	if len(dataSet.Inventory) > 0 {
		inventoryDataSet := BuildInventoryDataSet(elog, dataSet.Inventory, labels, integrationUser, pluginName, entityKey.String())
		emitter.EmitInventory(inventoryDataSet, entity.NewWithoutID(entityKey))
	}

	for _, metric := range dataSet.Metrics {
		if !dataSet.Entity.IsAgent() {
			if len(metric)+len(extraAnnotations) > maxEntityAttributeCount {
				k := lru.Key(entityKey)
				if _, ok := logLRU.Get(k); !ok {
					elog.
						WithField("entity", entityKey).
						Warn(entityMetricsLengthWarnMgs)
				}
				logLRU.Add(k, struct{}{})
			}
		}

		for key, value := range labels {
			metric[labelPrefix+key] = value
		}
		for key, value := range extraAnnotations {
			// Extra annotations can't override current metrics
			if _, ok := metric[key]; !ok {
				metric[key] = value
			}
		}
		if integrationUser != "" {
			metric["integrationUser"] = integrationUser
		}
		if metricEventType, ok := metric["event_type"]; ok {

			// We want to add displayName and entityName for remote entities in the agent in case these fields are missing
			if !dataSet.Entity.IsAgent() {
				if displayName, ok := metric["displayName"]; !ok || displayName == "" {
					metric["displayName"] = entityKey
				}
				if entityName, ok := metric["entityName"]; !ok || entityName == "" {
					metric["entityName"] = entityKey
				}
				if reportingAgent, ok := metric["reportingAgent"]; !ok || reportingAgent == "" {
					metric["reportingAgent"] = agentIdentifier
				}

				if reportingEndpoint, ok := metric["reportingEndpoint"]; ok {
					replacement, err := replaceLoopbackFromField(reportingEndpoint, idLookup, protocolVersion)
					if err != nil {
						elog.WithError(err).Warn("reportingEndpoint attribute replacement failed")
					} else {
						metric["reportingEndpoint"] = replacement
					}
				}
				if reportingEntityKey, ok := metric["reportingEntityKey"]; ok {
					replacement, err := replaceLoopbackFromField(reportingEntityKey, idLookup, protocolVersion)
					if err != nil {
						elog.WithError(err).Warn("reportingEntityKey attribute replacement failed")
					} else {
						metric["reportingEntityKey"] = replacement
					}
				}
			}
			metric["entityKey"] = entityKey

			// NOTE: The agent requires the eventType field for now
			metric["eventType"] = metricEventType

			metric["integrationName"] = pluginName
			metric["integrationVersion"] = pluginVersion

			// there are integrations that add the hostname so
			// Let's make sure that we do NOT have hostname in the metrics.
			delete(metric, "hostname")

			emitter.EmitEvent(metric, entityKey)
		} else {
			elog.WithIntegration(pluginName).WithField("metric", metric).Debug("Missing event_type field for metric.")
		}
	}

	for _, event := range dataSet.Events {
		normalizedEvent := NormalizeEvent(elog, event, labels, extraAnnotations, integrationUser, entityKey.String())

		if normalizedEvent != nil {
			emitter.EmitEvent(normalizedEvent, entityKey)
		}
	}

	return nil
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

func getVariantHash(entity map[string]interface{}) (hash string, err error) {
	entityBuf, err := json.Marshal(entity)
	if err != nil {
		return "", err
	}

	hashBytes := md5.Sum(entityBuf)
	hash = fmt.Sprintf("%x", hashBytes)
	return
}

// replaceLoopbackFromField will try to match and replace loopback address from a MetricData field.
func replaceLoopbackFromField(field interface{}, lookup host.IDLookup, protocol int) (string, error) {
	value, ok := field.(string)
	if !ok {
		return "", errors.New("can't replace loopback when the field is not a string")
	}
	return entity.ReplaceLoopback(value, lookup, protocol)
}
