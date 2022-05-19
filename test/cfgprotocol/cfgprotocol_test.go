// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || windows
// +build linux windows

package cfgprotocol

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/test/cfgprotocol/agent"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	timeout        = 20 * time.Second
	metricNRIOutV3 = `[{
			"ExternalKeys":["shell-test:some-entity"],
			"IsAgent":false,
			"Events":[
				{
					"displayName":"shell-test:some-entity","entityKey":"shell-test:some-entity","entityName":"shell-test:some-entity",
					"eventType":"ShellTestSample","event_type":"ShellTestSample","integrationName":"com.newrelic.shelltest","integrationVersion":"0.0.0",
					"reportingAgent":"my_display_name","some-metric":1
				}
			]
		}]`
	metricNRIOutV4 = `[{
			"ExternalKeys": ["uniqueName"],
			"IsAgent": false,
			"Events": [{"attr.format": "attribute","attributes": {"format": "attribute"},"category": "notifications","entityKey": "uniqueName",
					"eventType": "InfrastructureEvent","format": "event","summary": "foo"}]
			}]`
)

var concurrentAgent = make(chan interface{}, 1)

func lock() {
	concurrentAgent <- 1
}
func free() {
	<-concurrentAgent
}

func createAgentAndStart(t *testing.T, scenario string) *agent.Emulator {
	lock()
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(t, err)
	spawnerDir := filepath.Join("testdata", "go", "spawner.go")
	require.NoError(t, testhelp.GoBuild(testhelp.Script(spawnerDir), niDir))

	integrationsConfigPath := filepath.Join("testdata", "scenarios", scenario)
	a := agent.New(integrationsConfigPath, niDir)
	require.NoError(t, a.RunAgent())
	return a
}

func Test_OneIntegrationIsExecutedV4(t *testing.T) {
	a := createAgentAndStart(t, "v4_payload")
	defer a.Terminate()
	defer free()

	// the agent sends samples from the integration
	select {
	case req := <-a.ChannelHTTPRequests():
		bodyBuffer, _ := ioutil.ReadAll(req.Body)
		assertMetrics(t, metricNRIOutV4, string(bodyBuffer), []string{"timestamp"})
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
		return
	}

}

/**
Given a config protocol integration that spawns a short running process
When the integrations is executed
Then there is a child process running
When the short execution is terminated
Then there are not child processes
*/
func Test_OneIntegrationIsExecutedAndTerminated(t *testing.T) {
	a := createAgentAndStart(t, "scenario0")
	defer a.Terminate()
	defer free()

	// the agent sends samples from the integration
	select {
	case req := <-a.ChannelHTTPRequests():
		bodyBuffer, _ := ioutil.ReadAll(req.Body)
		if !assertMetrics(t, metricNRIOutV3, string(bodyBuffer), []string{"timestamp"}) {
			return
		}
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
		return
	}
	processNameRe := getProcessNameRegExp("nri-out-short")
	// and just one integrations process is running
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err := findChildrenProcessByCmdName(processNameRe)
		assert.NoError(rt, err)
		assert.Len(rt, p, 1)
	})

	// there are no process running
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err := findAllProcessByCmd(processNameRe)
		assert.NoError(rt, err)
		assert.Empty(rt, p)
	})
}

/**
Given a config protocol integration that spawns a long running process
When the long running process is killed
Then a new long running process with a new PID is launched
*/
func Test_IntegrationIsRelaunchedIfTerminated(t *testing.T) {
	a := createAgentAndStart(t, "scenario1")
	defer a.Terminate()
	defer free()
	// and just one integrations process is running
	var p []*process.Process
	var err error
	processName := getProcessNameRegExp("nri-out-long")
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err = findChildrenProcessByCmdName(processName)
		assert.NoError(rt, err)
		assert.Len(rt, p, 1)
	})
	// if the integration exits with error code
	require.NotNil(t, p[0])
	oldPid := p[0].Pid
	assert.NoError(t, p[0].Kill())
	// is eventually spawned again by the runner
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err = findAllProcessByCmd(processName)
		assert.NoError(rt, err)
		assert.Len(rt, p, 0)
	})
	var newPid = oldPid
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err = findAllProcessByCmd(processName)
		assert.NoError(rt, err)
		if assert.Len(rt, p, 1) {
			newPid = p[0].Pid
		}
	})
	assert.NotEqual(t, oldPid, newPid)
}

/**
Given a config protocol integration that spawns a long running process
When the configuration for the long running is updated
Then the running long process is killed
And a new long running process with a new PID is launched
*/
func Test_IntegrationIsRelaunchedIfIntegrationDetailsAreChanged(t *testing.T) {
	nriCfgTemplatePath := templatePath("nri-config.json")
	nriCfgPath := filepath.Join("testdata", "scenarios", "scenario2", "nri-config.json")
	assert.Nil(t, createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"timestamp":   time.Now(),
		"processName": "nri-out-process",
	}))
	a := createAgentAndStart(t, "scenario2")
	defer a.Terminate()
	defer free()

	// and just one integrations process is running
	var p []*process.Process
	var err error
	processNameRe := getProcessNameRegExp("nri-out-process")
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err = findChildrenProcessByCmdName(processNameRe)
		assert.NoError(rt, err)
		assert.Len(rt, p, 1)
	})
	// if the integration exits with error code
	require.NotNil(t, p[0])
	oldPid := p[0].Pid
	assert.Nil(t, createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"timestamp":   time.Now(),
		"processName": "nri-out-process",
	}))
	testhelpers.Eventually(t, 25*time.Second, func(rt require.TestingT) {
		p, err = findAllProcessByCmd(processNameRe)
		assert.NoError(rt, err)
		if assert.Len(rt, p, 1) {
			assert.NotEqual(rt, oldPid, p[0].Pid)
		}
	})
	assert.Len(t, p, 1)
}

/**
Given a config protocol integration that spawns two long running process
When one of the spawn integrations is removed
Then one process continue running with the same PID
And the other process is removed
*/
func Test_IntegrationConfigContainsTwoIntegrationsAndOneIsRemoved(t *testing.T) {
	nriCfgTemplatePath := templatePath("nri-config-two-integrations.json")
	nriCfgPath := filepath.Join("testdata", "scenarios", "scenario3", "nri-config.json")
	assert.Nil(t, createFile(nriCfgTemplatePath, nriCfgPath, nil))
	a := createAgentAndStart(t, "scenario3")
	defer a.Terminate()
	defer free()
	// and just one integrations process is running
	var p1 []*process.Process
	var p2 []*process.Process
	var err error

	processName1Re := getProcessNameRegExp("nri-out-long-1")
	processName2Re := getProcessNameRegExp("nri-out-long-2")
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p1, err = findChildrenProcessByCmdName(processName1Re)
		assert.NoError(rt, err)
		assert.Len(rt, p1, 1)
		p2, err = findChildrenProcessByCmdName(processName2Re)
		assert.NoError(rt, err)
		assert.Len(rt, p2, 1)
	})
	require.NotNil(t, p1[0])
	p1OldPid := p1[0].Pid

	nriCfgTemplatePath = templatePath("nri-config.json")
	assert.Nil(t, createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"processName": "nri-out-long-1",
	}))

	testhelpers.Eventually(t, 40*time.Second, func(rt require.TestingT) {
		p1, err := findChildrenProcessByCmdName(processName1Re)
		assert.NoError(rt, err)
		if assert.Len(rt, p1, 1) {
			assert.Equal(rt, p1[0].Pid, p1OldPid)
		}
		p2, err := findChildrenProcessByCmdName(processName2Re)
		assert.NoError(rt, err)
		assert.Len(rt, p2, 0)
	})
}

/**
Given a config protocol integration that generates 2 differnet configs with the same integrations
When the configuration file of the spawner is removed
Then all running integrations are terminated
*/
func Test_IntegrationConfigNewRelicInfraConfigurationIsRemoved(t *testing.T) {
	nriCfgTemplatePath := templatePath("settings.yml")
	nriCfgPath := filepath.Join("testdata", "scenarios", "scenario4", "settings.yml")
	assert.Nil(t, createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"scenario": "scenario4",
	}))
	a := createAgentAndStart(t, "scenario4")
	defer a.Terminate()
	defer free()
	processNameRe := getProcessNameRegExp("nri-out-long-4")
	var p []*process.Process
	var err error
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err = findChildrenProcessByCmdName(processNameRe)
		assert.NoError(rt, err)
		assert.Len(rt, p, 2)
	})
	assert.Nil(t, os.Remove(nriCfgPath))
	testhelpers.Eventually(t, timeout, func(rt require.TestingT) {
		p, err = findAllProcessByCmd(processNameRe)
		assert.NoError(rt, err)
		assert.Len(rt, p, 0)
	})
}

/**
Given a config protocol integration that spawns an integration that contains a config entry
When the integration is spawned
Then receives the temporary generated config file path is passed to the integration
*/
func Test_IntegrationConfigContainsConfigTemplate(t *testing.T) {
	a := createAgentAndStart(t, "scenario5")
	defer a.Terminate()
	defer free()

	// the agent sends samples from the integration
	select {
	case req := <-a.ChannelHTTPRequests():
		bodyBuffer, _ := ioutil.ReadAll(req.Body)
		assertMetrics(t, metricNRIOutV3, string(bodyBuffer), []string{"timestamp"})
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
		return
	}
}
