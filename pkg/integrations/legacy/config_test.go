// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type ConfigSuite struct{}

var _ = Suite(&ConfigSuite{})

func (self *ConfigSuite) TestLoadConfig(c *C) {
	pluginsYaml, err := LoadPluginConfig(&PluginRegistry{}, []string{"fixtures/simplePlugins.yaml"})
	if err != nil {
		c.Fatal(err)
	}

	c.Assert(len(pluginsYaml.PluginConfigs), Equals, 1)
	c.Assert(pluginsYaml.PluginConfigs[0].PluginName, Equals, "ls-root")
	c.Assert(len(pluginsYaml.PluginConfigs[0].PluginInstances), Equals, 2)
	c.Assert(pluginsYaml.PluginConfigs[0].PluginInstances[0]["directoryToMonitor"], Equals, "/home")
	c.Assert(pluginsYaml.PluginConfigs[0].PluginInstances[1]["directoryToMonitor"], Equals, "/home/matt")
}
