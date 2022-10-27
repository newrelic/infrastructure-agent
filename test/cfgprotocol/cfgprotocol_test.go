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
	"github.com/stretchr/testify/suite"
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

type CfgProtocolTestSuite struct {
	suite.Suite
}

func TestCfgProtocolTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CfgProtocolTestSuite))
}

// Only allow one agent to run at a time. Lock before test.
func (suite *CfgProtocolTestSuite) BeforeTest(_, _ string) {
	lock()
}

// Only allow one agent to run at a time. Free after test.
func (suite *CfgProtocolTestSuite) AfterTest(_, _ string) {
	free()
}

func (suite *CfgProtocolTestSuite) createAgentAndStart(scenario string) *agent.Emulator {
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(suite.T(), err)
	spawnerDir := filepath.Join("testdata", "go", "spawner.go")
	require.NoError(suite.T(), testhelp.GoBuild(testhelp.Script(spawnerDir), niDir))

	integrationsConfigPath := filepath.Join("testdata", "scenarios", scenario)
	a := agent.New(integrationsConfigPath, niDir)
	require.NoError(suite.T(), a.RunAgent())
	return a
}

func (suite *CfgProtocolTestSuite) Test_OneIntegrationIsExecutedV4() {
	a := suite.createAgentAndStart("v4_payload")
	defer a.Terminate()

	// the agent sends samples from the integration
	select {
	case req := <-a.ChannelHTTPRequests():
		bodyBuffer, _ := ioutil.ReadAll(req.Body)
		assertMetrics(suite.T(), metricNRIOutV4, string(bodyBuffer), []string{"timestamp"})
	case <-time.After(timeout):
		assert.FailNow(suite.T(), "timeout while waiting for a response")
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
func (suite *CfgProtocolTestSuite) Test_OneIntegrationIsExecutedAndTerminated() {
	a := suite.createAgentAndStart("scenario0")
	defer a.Terminate()

	// the agent sends samples from the integration
	select {
	case req := <-a.ChannelHTTPRequests():
		bodyBuffer, _ := ioutil.ReadAll(req.Body)
		if !assertMetrics(suite.T(), metricNRIOutV3, string(bodyBuffer), []string{"timestamp"}) {
			return
		}
	case <-time.After(timeout):
		assert.FailNow(suite.T(), "timeout while waiting for a response")
		return
	}

	processNameRe := getProcessNameRegExp("nri-out-short")
	// and just one integrations process is running
	testhelpers.Eventually(suite.T(), timeout, func(reqT require.TestingT) {
		p, err := findChildrenProcessByCmdName(processNameRe)
		assert.NoError(reqT, err)
		assert.Len(reqT, p, 1)
	})

	// there are no process running
	testhelpers.Eventually(suite.T(), timeout, func(reqT require.TestingT) {
		p, err := findAllProcessByCmd(processNameRe)
		assert.NoError(reqT, err)
		assert.Empty(reqT, p)
	})
}

/**
Given a config protocol integration that spawns a long running process
When the long running process is killed
Then a new long running process with a new PID is launched
*/
func (suite *CfgProtocolTestSuite) Test_IntegrationIsRelaunchedIfTerminated() {
	a := suite.createAgentAndStart("scenario1")
	defer a.Terminate()
	// and just one integrations process is running
	var p []*process.Process
	var err error

	processName := getProcessNameRegExp("nri-out-long")

	testhelpers.Eventually(suite.T(), timeout, func(reqt require.TestingT) {
		p, err = findChildrenProcessByCmdName(processName)
		assert.NoError(reqt, err)
		assert.Len(reqt, p, 1)
	})
	// if the integration exits with error code
	require.NotNil(suite.T(), p[0])
	oldPid := p[0].Pid
	assert.NoError(suite.T(), p[0].Kill())

	// is eventually spawned again by the runner
	testhelpers.Eventually(suite.T(), timeout, func(reqt require.TestingT) {
		p, err = findAllProcessByCmd(processName)
		assert.NoError(reqt, err)
		assert.Len(reqt, p, 0)
	})

	newPid := oldPid

	testhelpers.Eventually(suite.T(), timeout, func(reqt require.TestingT) {
		p, err = findAllProcessByCmd(processName)
		assert.NoError(reqt, err)
		if assert.Len(reqt, p, 1) {
			newPid = p[0].Pid
		}
	})
	assert.NotEqual(suite.T(), oldPid, newPid)
}

/**
Given a config protocol integration that spawns a long running process
When the configuration for the long running is updated
Then the running long process is killed
And a new long running process with a new PID is launched
*/
func (suite *CfgProtocolTestSuite) Test_IntegrationIsRelaunchedIfIntegrationDetailsAreChanged() {
	nriCfgTemplatePath := templatePath("nri-config.json")
	nriCfgPath := filepath.Join("testdata", "scenarios", "scenario2", "nri-config.json")
	assert.Nil(suite.T(), createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"timestamp":   time.Now(),
		"processName": "nri-out-process",
	}))
	a := suite.createAgentAndStart("scenario2")
	defer a.Terminate()

	// and just one integrations process is running
	var p []*process.Process
	var err error

	processNameRe := getProcessNameRegExp("nri-out-process")

	testhelpers.Eventually(suite.T(), timeout, func(reqt require.TestingT) {
		p, err = findChildrenProcessByCmdName(processNameRe)
		assert.NoError(reqt, err)
		assert.Len(reqt, p, 1)
	})

	// if the integration exits with error code
	require.NotNil(suite.T(), p[0])
	oldPid := p[0].Pid

	assert.Nil(suite.T(), createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"timestamp":   time.Now(),
		"processName": "nri-out-process",
	}))

	testhelpers.Eventually(suite.T(), 25*time.Second, func(reqt require.TestingT) {
		p, err = findAllProcessByCmd(processNameRe)
		assert.NoError(reqt, err)
		if assert.Len(reqt, p, 1) {
			assert.NotEqual(reqt, oldPid, p[0].Pid)
		}
	})
	assert.Len(suite.T(), p, 1)
}

/**
Given a config protocol integration that spawns two long running process
When one of the spawn integrations is removed
Then one process continue running with the same PID
And the other process is removed
*/
func (suite *CfgProtocolTestSuite) Test_IntegrationConfigContainsTwoIntegrationsAndOneIsRemoved() {
	nriCfgTemplatePath := templatePath("nri-config-two-integrations.json")
	nriCfgPath := filepath.Join("testdata", "scenarios", "scenario3", "nri-config.json")
	assert.Nil(suite.T(), createFile(nriCfgTemplatePath, nriCfgPath, nil))
	a := suite.createAgentAndStart("scenario3")
	defer a.Terminate()
	// and just one integrations process is running
	var p1 []*process.Process
	var p2 []*process.Process
	var err error

	processName1Re := getProcessNameRegExp("nri-out-long-1")
	processName2Re := getProcessNameRegExp("nri-out-long-2")

	testhelpers.Eventually(suite.T(), timeout, func(reqt require.TestingT) {
		p1, err = findChildrenProcessByCmdName(processName1Re)
		assert.NoError(reqt, err)
		assert.Len(reqt, p1, 1)
		p2, err = findChildrenProcessByCmdName(processName2Re)
		assert.NoError(reqt, err)
		assert.Len(reqt, p2, 1)
	})
	require.NotNil(suite.T(), p1[0])
	p1OldPid := p1[0].Pid

	nriCfgTemplatePath = templatePath("nri-config.json")
	assert.Nil(suite.T(), createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"processName": "nri-out-long-1",
	}))

	testhelpers.Eventually(suite.T(), 40*time.Second, func(reqT require.TestingT) {
		p1, err := findChildrenProcessByCmdName(processName1Re)
		assert.NoError(reqT, err)
		if assert.Len(reqT, p1, 1) {
			assert.Equal(reqT, p1[0].Pid, p1OldPid)
		}
		p2, err := findChildrenProcessByCmdName(processName2Re)
		assert.NoError(reqT, err)
		assert.Len(reqT, p2, 0)
	})
}

/**
Given a config protocol integration that generates 2 differnet configs with the same integrations
When the configuration file of the spawner is removed
Then all running integrations are terminated
*/
func (suite *CfgProtocolTestSuite) Test_IntegrationConfigNewRelicInfraConfigurationIsRemoved() {
	nriCfgTemplatePath := templatePath("settings.yml")
	nriCfgPath := filepath.Join("testdata", "scenarios", "scenario4", "settings.yml")
	assert.Nil(suite.T(), createFile(nriCfgTemplatePath, nriCfgPath, map[string]interface{}{
		"scenario": "scenario4",
	}))
	a := suite.createAgentAndStart("scenario4")
	defer a.Terminate()
	processNameRe := getProcessNameRegExp("nri-out-long-4")
	var p []*process.Process
	var err error
	testhelpers.Eventually(suite.T(), timeout, func(reqt require.TestingT) {
		p, err = findChildrenProcessByCmdName(processNameRe)
		assert.NoError(reqt, err)
		assert.Len(reqt, p, 2)
	})
	assert.Nil(suite.T(), os.Remove(nriCfgPath))
	testhelpers.Eventually(suite.T(), timeout, func(reqt require.TestingT) {
		p, err = findAllProcessByCmd(processNameRe)
		assert.NoError(reqt, err)
		assert.Len(reqt, p, 0)
	})
}

/**
Given a config protocol integration that spawns an integration that contains a config entry
When the integration is spawned
Then receives the temporary generated config file path is passed to the integration
*/
func (suite *CfgProtocolTestSuite) Test_IntegrationConfigContainsConfigTemplate() {
	a := suite.createAgentAndStart("scenario5")
	defer a.Terminate()

	// the agent sends samples from the integration
	select {
	case req := <-a.ChannelHTTPRequests():
		bodyBuffer, _ := ioutil.ReadAll(req.Body)
		assertMetrics(suite.T(), metricNRIOutV3, string(bodyBuffer), []string{"timestamp"})
	case <-time.After(timeout):
		assert.FailNow(suite.T(), "timeout while waiting for a response")
		return
	}
}
