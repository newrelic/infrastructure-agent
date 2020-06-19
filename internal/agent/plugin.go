// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
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

// Anything implementing the sortable interface must implement a
// method to return a string Sort key
type Sortable interface {
	SortKey() string
}

type PluginInventoryDataset []Sortable // PluginInventoryDataset is a slice of sortable things

// PluginInventoryDataset also implements the sort.Sort interface
func (pd PluginInventoryDataset) Len() int           { return len(pd) }
func (pd PluginInventoryDataset) Swap(i, j int)      { pd[i], pd[j] = pd[j], pd[i] }
func (pd PluginInventoryDataset) Less(i, j int) bool { return pd[i].SortKey() < pd[j].SortKey() }

// PluginOutput contains metadata about the inventory provided by Plugins, which will be used for its later addition
// to the delta store
type PluginOutput struct {
	Id            ids.PluginID
	EntityKey     string
	Data          PluginInventoryDataset
	NotApplicable bool
}

func NewPluginOutput(id ids.PluginID, entityKey string, data PluginInventoryDataset) PluginOutput {
	return PluginOutput{Id: id, EntityKey: entityKey, Data: data}
}

func NewNotApplicableOutput(id ids.PluginID) PluginOutput {
	return PluginOutput{Id: id, NotApplicable: true}
}

// Id is the accessor for the id field
func (self *PluginCommon) Id() ids.PluginID {
	return self.ID
}

// IsExternal is the accessor for the External field
func (self *PluginCommon) IsExternal() bool {
	return self.External
}

// LogInfo retrieves logs the plugin name for internal plugins, and
// for the external plugins it logs the data specified in the log fields.
func (self *PluginCommon) LogInfo() {
	if self.IsExternal() {
		log.WithFieldsF(self.DetailedLogFields).Info("Integration info")
	} else {
		log.WithPlugin(self.Id().String()).Info("Agent plugin")
	}
}

// GetExternalPluginName is the accessor for the ExternalPluginName field
func (self *PluginCommon) GetExternalPluginName() string {
	return self.ExternalPluginName
}

type PluginEmitter interface {
	EmitInventory(data PluginInventoryDataset, entityKey string)
	EmitEvent(eventData map[string]interface{}, entityKey entity.Key)
}

// EmitInventory sends data collected by the plugin to the agent
func (self *PluginCommon) EmitInventory(data PluginInventoryDataset, entityKey string) {
	self.Context.SendData(NewPluginOutput(self.ID, entityKey, data))
}

func (self *PluginCommon) EmitEvent(eventData map[string]interface{}, entityKey entity.Key) {
	self.decorateEvent(eventData)
	self.Context.SendEvent(mapEvent(eventData), entityKey)
}

func (self *PluginCommon) gatherDecorations() {
	self.once.Do(func() {
		cfg := self.Context.Config()
		if cfg != nil && cfg.K8sIntegration {
			self.decorations = metadata.GatherK8sMetadata()
		}
	})
}

func (self *PluginCommon) decorateEvent(eventData map[string]interface{}) {
	eventData["timestamp"] = time.Now().Unix()

	self.gatherDecorations()
	for k, v := range self.decorations {
		eventData[k] = v
	}
}

// Unregister tells the agent that this plugin cannot run
func (self *PluginCommon) Unregister() {
	self.Context.Unregister(self.Id())
}

func (self *PluginCommon) ScheduleHealthCheck() {
	if !self.IsExternal() {
		return
	}

	// The health check channel has a size of 1 so if writing to it blocks
	// it means a health check has already been scheduled.
	select {
	case self.HealthCheckCh <- struct{}{}:
		log.WithFields(self.LogFields).Info("Integration health check scheduled")
	default:
		log.WithFields(self.LogFields).Info("Integration health check already requested")
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
