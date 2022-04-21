// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testemit

import (
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v3/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/dm"
	protocol2 "github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/protocol"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/integration"
)

const (
	channelBuffer  = 100
	receiveTimeout = 2000000 * time.Second
)

// EmittedData stores both PluginDataSetV3 metric and the plugin metadata
type EmittedData struct {
	DataSet       protocol2.PluginDataSetV3
	Metadata      integration.Definition
	ExtraLabels   data.Map
	EntityRewrite []data.EntityRewrite
}

// RecordEmitter implements a test emitter that stores the submitted data as Plugins structs
type RecordEmitter struct {
	received map[string]chan EmittedData
	mutex    sync.Mutex
}

func (t *RecordEmitter) Emit(metadata integration.Definition, extraLabels data.Map, entityRewrite []data.EntityRewrite, json []byte) error {
	protocolVersion, err := protocol2.VersionFromPayload(json, true)
	if err != nil {
		return err
	}

	// dimensional metrics
	if protocolVersion == protocol2.V4 {
		ffMan := feature_flags.NewManager(map[string]bool{fflag.FlagProtocolV4: true})
		data, err := dm.ParsePayloadV4(json, ffMan)
		if err != nil {
			return err
		}
		ch := t.channelFor(metadata.Name)
		for _, ds := range data.DataSets {
			ch <- EmittedData{
				DataSet: protocol2.PluginDataSetV3{PluginDataSet: protocol2.PluginDataSet{
					Entity: ds.Entity,
					// TODO but for now it's enough for the assertion mechanism:
					Metrics: make([]protocol2.MetricData, len(ds.Metrics)),
				}},
				Metadata:      metadata,
				ExtraLabels:   extraLabels,
				EntityRewrite: entityRewrite,
			}
		}
		return nil
	}

	data, _, err := config.ParsePayload(json, false)
	if err != nil {
		return err
	}
	ch := t.channelFor(metadata.Name)
	for _, ds := range data.DataSets {
		ch <- EmittedData{
			DataSet:       ds,
			Metadata:      metadata,
			ExtraLabels:   extraLabels,
			EntityRewrite: entityRewrite,
		}
	}

	return nil
}

func (t *RecordEmitter) ReceiveFrom(pluginName string) (EmittedData, error) {
	select {
	case dataset := <-t.channelFor(pluginName):
		return dataset, nil
	case <-time.After(receiveTimeout):
		return EmittedData{}, errors.New("timeout receiving payloads from plugin " + pluginName)
	}
}

// ExpectTimeout returns error if no timeout happens when listening for the plugin emissions.
// It embeds the emitted metric information in the error, if it has received it
func (t *RecordEmitter) ExpectTimeout(pluginName string, timeout time.Duration) error {
	select {
	case dataset := <-t.channelFor(pluginName):
		return fmt.Errorf("metrics were not expected. Received: %+v", dataset)
	case <-time.After(timeout):
		return nil
	}
}

func (t *RecordEmitter) channelFor(pluginName string) chan EmittedData {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.received == nil {
		t.received = map[string]chan EmittedData{}
	}
	channel, ok := t.received[pluginName]
	if !ok {
		channel = make(chan EmittedData, channelBuffer)
		t.received[pluginName] = channel
	}
	return channel
}
