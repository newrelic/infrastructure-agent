// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	testHelper "github.com/newrelic/infrastructure-agent/internal/plugins/testing"
	. "gopkg.in/check.v1"
)

// Register test suite
func TestFacter(t *testing.T) {
	TestingT(t)
}

type FacterSuite struct {
	agent *testHelper.MockAgent
}

var _ = Suite(&FacterSuite{})

func (s *FacterSuite) SetUpSuite(c *C) {
}

func (s *FacterSuite) TearDownSuite(c *C) {
}

func (s *FacterSuite) SetUpTest(c *C) {
	s.agent = testHelper.NewMockAgent()
}

func (s *FacterSuite) TestGetFacterData(c *C) {
	ignored_facts := []string{"uptime", "memoryfree"}

	badCmdOutput := `
Could not retrieve fact='hostname', resolution='<anonymous>': Could not execute 'hostname': command not found
Could not retrieve fact='hostname', resolution='<anonymous>': Could not execute 'hostname': command not found
`

	cmdOutput := `
{
  "hardwareisa": "x86_64",
  "kernel": "Linux",
  "uptime": "15:49 hours",
  "selinux": false,
  "memoryfree": ""
}
`
	var facterOutput map[string]interface{}

	err := json.Unmarshal([]byte(badCmdOutput), &facterOutput)
	c.Assert(err, NotNil)

	err = json.Unmarshal([]byte(cmdOutput), &facterOutput)
	c.Assert(err, Equals, nil)
	facterItems := buildFilteredMap(facterOutput, ignored_facts)

	//test FilterItem objects are showing up with the right values
	c.Assert(facterItems["hardwareisa"].Value, Equals, "x86_64")
	c.Assert(facterItems["kernel"].Value, Equals, "Linux")

	//test filters are working
	c.Assert(facterItems["uptime"].Value, Equals, nil)
	c.Assert(facterItems["memoryfree"].Value, Equals, nil)
}

func (s *FacterSuite) TestParseFacter(c *C) {
	fixture, err := ioutil.ReadFile("./fixtures/facter/iam_date.json")
	c.Assert(err, IsNil)
	facts, err := parseFacts(fixture)
	c.Assert(err, IsNil)
	fact := facts["ec2_metadata/iam/info"]
	c.Assert(strings.Contains(fact.Value.(string), "timestamp suppressed"), Equals, true)

}

func (s *FacterSuite) NewPlugin(facter Facter, c *C) *FacterPlugin {
	plugin := NewFacterPlugin(s.agent)
	plugin.frequency = 1 * time.Millisecond
	plugin.facter = facter
	go plugin.Run()
	return plugin
}

type MockFacter struct {
}

func (self *MockFacter) Initialize() error {
	return nil
}

func (self *MockFacter) Facts() (map[string]FacterItem, error) {
	key := "/partitions/sda1/filesystem"
	fact := FacterItem{
		Name:  key,
		Value: "ext2",
	}
	facts := make(map[string]FacterItem)
	facts[key] = fact
	return facts, nil
}

type FacterInitError struct {
}

func (self *FacterInitError) Initialize() error {
	return fmt.Errorf("Bummer!")
}

func (self *FacterInitError) Facts() (map[string]FacterItem, error) {
	return nil, fmt.Errorf("Bummer!")
}

type FacterNoFacts struct {
}

func (self *FacterNoFacts) Initialize() error {
	return nil
}

func (self *FacterNoFacts) Facts() (map[string]FacterItem, error) {
	return nil, fmt.Errorf("Bummer!")
}
