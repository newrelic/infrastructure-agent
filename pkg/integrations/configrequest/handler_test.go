// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package configrequest

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/cache"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	cfgprotocol "github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	commonConfig = `{
		"config_protocol_version": "1",
		"config_name": "config-name",
		"action": "action-name",
		"config": { "integrations": [ %s ] } 
		}`
)

func Test_addAndRemoveDefinitions(t *testing.T) {
	// Given a Clean cache and queues for the handle function
	configProtocolQueue := make(chan Entry, 10)
	terminateDefinitionQueue := make(chan string, 10)
	il := integration.InstancesLookup{
		ByName: func(_ string) (string, error) {
			return "/path/to/nri-process-discovery", nil
		},
	}
	var logger log.Entry
	handleFunction := NewHandleFn(configProtocolQueue, terminateDefinitionQueue, il, logger)
	c := cache.CreateCache()
	// And a config protocol request with two integrations
	firstPayload := []byte(fmt.Sprintf(commonConfig, `{ "name": "nri-1"}, { "name": "nri-2"} `))
	cp1, err := cfgprotocol.GetConfigProtocolBuilder(firstPayload).Build()
	assert.NoError(t, err)

	expectedLabels := map[string]string{"env": "test"}
	parentDefinition := integration.Definition{
		Labels: expectedLabels,
	}

	// When is processed by the handle function
	handleFunction(cp1, c, parentDefinition)

	// Then the two integrations are sent to the queue for being executed and no runner is terminated
	assert.Len(t, configProtocolQueue, 2)
	assert.Len(t, terminateDefinitionQueue, 0)

	for len(configProtocolQueue) > 0 {
		subDefinition := <-configProtocolQueue
		assert.Equal(t, subDefinition.Definition.Labels, expectedLabels)
	}

	// Given the cache with the previous integrations loaded and a second payload of cfg request
	// having 1 new integration 1 removed
	secondPayload := []byte(fmt.Sprintf(commonConfig, `{ "name": "nri-1"}, { "name": "nri-3"} `))
	cp2, err := cfgprotocol.GetConfigProtocolBuilder(secondPayload).Build()
	assert.NoError(t, err)
	// When the handle function is executed again
	handleFunction(cp2, c, parentDefinition)

	// then just 1 is executed and 1 removed
	assert.Len(t, configProtocolQueue, 1)

	for len(configProtocolQueue) > 0 {
		subDefinition := <-configProtocolQueue
		assert.Equal(t, subDefinition.Definition.Labels, expectedLabels)
	}

	assert.Len(t, terminateDefinitionQueue, 1)

}
func Test_failedToAddDefinition(t *testing.T) {
	// Given a Clean cache and queues for the handle function
	logger := log.WithComponent("LogTester")
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)
	configProtocolQueue := make(chan Entry, 10)
	terminateDefinitionQueue := make(chan string, 10)
	il := integration.InstancesLookup{
		ByName: func(_ string) (string, error) {
			return "", fmt.Errorf("fail")
		},
	}
	handleFunction := NewHandleFn(configProtocolQueue, terminateDefinitionQueue, il, logger)
	c := cache.CreateCache()

	// And a config protocol request
	payload := []byte(fmt.Sprintf(commonConfig, `{ "name": "nri-1"}`))
	cp1, err := cfgprotocol.GetConfigProtocolBuilder(payload).Build()
	assert.NoError(t, err)

	// When is processed by the handle function
	handleFunction(cp1, c, integration.Definition{})

	// Then handleFunction fails to process the defintion
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Equal(t, logFailedDefinition, hook.LastEntry().Message)

}
