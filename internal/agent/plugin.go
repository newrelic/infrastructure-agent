// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	goContext "context"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/instrumentation"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/internal/agent/metadata"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

// Plugin describes the interface all agent plugins implement
type Plugin interface {
	Run()
	Id() ids.PluginID
	IsExternal() bool
	GetExternalPluginName() string
	LogInfo()
	ScheduleHealthCheck()
}

// Killable defines the behaviour of a plugin that can be externally terminated.
type Killable interface {
	// Kill terminates the receiver of the function.
	Kill()
}

// PluginCommon contains attributes and methods available to all plugins
type PluginCommon struct {
	ID                 ids.PluginID // the "ID" is the path we write the json to
	Context            AgentContext // a reference to the calling agent context
	External           bool         // If the plugin is an external plugin
	ExternalPluginName string       // The external plugin name. Ex: com.newrelic.nginx
	// Returns all the information related to the plugin, including
	// runtime environment variables
	DetailedLogFields func() logrus.Fields
	LogFields         logrus.Fields // fields to include when logging about the plugin
	HealthCheckCh     chan struct{} // notifies the plugin to execute a health check
	decorations       map[string]interface{}
	once              sync.Once
}

func NewExternalPluginCommon(id ids.PluginID, ctx AgentContext, name string) PluginCommon {
	return PluginCommon{
		ID:                 id,
		Context:            ctx,
		External:           true,
		ExternalPluginName: name,
		HealthCheckCh:      make(chan struct{}, 1),
	}
}

// Id is the accessor for the id field
func (pc *PluginCommon) Id() ids.PluginID {
	return pc.ID
}

// IsExternal is the accessor for the External field
func (pc *PluginCommon) IsExternal() bool {
	return pc.External
}

// LogInfo retrieves logs the plugin name for internal plugins, and
// for the external plugins it logs the data specified in the log fields.
func (pc *PluginCommon) LogInfo() {
	if pc.IsExternal() {
		log.WithFieldsF(pc.DetailedLogFields).Info("Integration info")
	} else {
		log.WithPlugin(pc.Id().String()).Info("Agent plugin")
	}
}

// GetExternalPluginName is the accessor for the ExternalPluginName field
func (pc *PluginCommon) GetExternalPluginName() string {
	return pc.ExternalPluginName
}

type PluginEmitter interface {
	EmitInventory(data types.PluginInventoryDataset, entity entity.Entity)
	EmitEvent(eventData map[string]interface{}, entityKey entity.Key)
}

// EmitInventory sends data collected by the plugin to the agent
func (pc *PluginCommon) EmitInventory(data types.PluginInventoryDataset, entity entity.Entity) {
	_, txn := instrumentation.SelfInstrumentation.StartTransaction(goContext.Background(), "plugin.emit_inventory")
	txn.AddAttribute("plugin_id", fmt.Sprintf("%s:%s", pc.ID.Category, pc.ID.Term))
	defer txn.End()
	pc.Context.SendData(types.NewPluginOutput(pc.ID, entity, data))
}

func (pc *PluginCommon) EmitEvent(eventData map[string]interface{}, entityKey entity.Key) {
	pc.decorateEvent(eventData)
	pc.Context.SendEvent(mapEvent(eventData), entityKey)
}

func (pc *PluginCommon) gatherDecorations() {
	pc.once.Do(func() {
		cfg := pc.Context.Config()
		if cfg != nil && cfg.K8sIntegration {
			pc.decorations = metadata.GatherK8sMetadata()
		}
	})
}

func (pc *PluginCommon) decorateEvent(eventData map[string]interface{}) {
	if eventData["timestamp"] == nil {
		eventData["timestamp"] = time.Now().Unix()
	}

	pc.gatherDecorations()
	for k, v := range pc.decorations {
		eventData[k] = v
	}
}

// Unregister tells the agent that this plugin cannot run
func (pc *PluginCommon) Unregister() {
	pc.Context.Unregister(pc.Id())
}

func (pc *PluginCommon) ScheduleHealthCheck() {
	if !pc.IsExternal() {
		return
	}

	// The health check channel has a size of 1 so if writing to it blocks
	// it means a health check has already been scheduled.
	select {
	case pc.HealthCheckCh <- struct{}{}:
		log.WithFields(pc.LogFields).Info("Integration health check scheduled")
	default:
		log.WithFields(pc.LogFields).Info("Integration health check already requested")
	}
}

// mapEvent allows the eventDataMap to fulfill the Event interface
type mapEvent map[string]interface{}

func (m mapEvent) Timestamp(timestamp int64) {
	m["timestamp"] = timestamp
}

func (m mapEvent) Type(eventType string) {
	m["eventType"] = eventType
}

func (m mapEvent) Entity(key entity.Key) {
	m["entityKey"] = key
}
