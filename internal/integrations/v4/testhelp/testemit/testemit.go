// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testemit

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	data2 "github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

const (
	channelBuffer  = 100
	receiveTimeout = 2000000 * time.Second
)

// EmittedData stores both PluginDataSetV3 metric and the plugin metadata
type EmittedData struct {
	DataSet       protocol.PluginDataSetV3
	Metadata      integration.Definition
	ExtraLabels   data2.Map
	EntityRewrite []data2.EntityRewrite
}

// RecordEmitter implements a test emitter that stores the submitted data as Plugins structs
type RecordEmitter struct {
	received map[string]chan EmittedData
	mutex    sync.Mutex
}

func (t *RecordEmitter) Emit(metadata integration.Definition, extraLabels data2.Map, entityRewrite []data2.EntityRewrite, json []byte) error {
	data, _, err := legacy.ParsePayload(json, false)
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
