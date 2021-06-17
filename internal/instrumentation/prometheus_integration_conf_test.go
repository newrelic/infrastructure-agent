// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestSetupPrometheusIntegrationConfig_CreateConfFile(t *testing.T) {

	promIntConf = `
			# TYPE newrelic_infra_instrumentation_dm_requests_forwarded counter
			integrations:
          		urls: ["{{.AgentMetricsEndpoint}}"]
				some_extra: "content"	
	`

	expectedConf := `
			# TYPE newrelic_infra_instrumentation_dm_requests_forwarded counter
			integrations:
          		urls: ["some-endpoint:2222"]
				some_extra: "content"	
	`

	ctx := context.Background()
	endpoint := "some-endpoint:2222"
	promIntConfPath = promIntConfPathTest()
	os.Remove(promIntConfPath)

	err := SetupPrometheusIntegrationConfig(ctx, endpoint)
	assert.Nil(t, err)
	assert.FileExists(t, promIntConfPath)
	data, err := ioutil.ReadFile(promIntConfPath)
	assert.Equal(t, expectedConf, string(data))
}

func TestSetupPrometheusIntegrationConfig_OverrideExistingFile(t *testing.T) {

	err := ioutil.WriteFile(promIntConfPathTest(), []byte("some random content"), 0755)
	assert.Nil(t, err)

	promIntConf = `
			# TYPE newrelic_infra_instrumentation_dm_requests_forwarded counter
			integrations:
          		urls: ["{{.AgentMetricsEndpoint}}"]
				some_extra: "content"	
	`

	expectedConf := `
			# TYPE newrelic_infra_instrumentation_dm_requests_forwarded counter
			integrations:
          		urls: ["some-endpoint:2222"]
				some_extra: "content"	
	`

	ctx := context.Background()

	endpoint := "some-endpoint:2222"
	promIntConfPath = os.TempDir() + "/TestSetupPrometheusIntegrationConfig.yml"

	err = SetupPrometheusIntegrationConfig(ctx, endpoint)
	assert.Nil(t, err)
	assert.FileExists(t, promIntConfPath)
	data, err := ioutil.ReadFile(promIntConfPath)
	assert.Equal(t, expectedConf, string(data))
}

func TestSetupPrometheusIntegrationConfig_DeleteConfFileOnFinish(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.Background())
	endpoint := "some-endpoint:2222"
	promIntConfPath = promIntConfPathTest()
	os.Remove(promIntConfPath)

	err := SetupPrometheusIntegrationConfig(ctx, endpoint)
	assert.Nil(t, err)

	cancelFn()
	//TODO find a better way of testing cancelFn w/o sleep as it makes it flacky and slow
	time.Sleep(time.Millisecond * 50)
	assert.NoFileExists(t, promIntConfPath)
}

func promIntConfPathTest() string {
	return os.TempDir() + "/TestSetupPrometheusIntegrationConfig.yml"
}
