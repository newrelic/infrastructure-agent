// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"
)

type RegistrySuite struct{}

var _ = Suite(&RegistrySuite{})

func (s *RegistrySuite) TestNewPluginRegistry(c *C) {
	r := NewPluginRegistry([]string{}, []string{})
	c.Assert(r, NotNil)
}

func MakePluginV1Dirs(name string, definitionData []byte, configData []byte) (dirs []string, err error) {
	definitionsDir, err := ioutil.TempDir("", "plugin-definitions")
	configsDir, err := ioutil.TempDir("", "plugin-configs")
	if err != nil {
		return
	}
	err = ioutil.WriteFile(filepath.Join(definitionsDir, fmt.Sprintf("%s.%s", name, "yaml")), definitionData, 0644)
	err = ioutil.WriteFile(filepath.Join(configsDir, fmt.Sprintf("%s.%s", name, "yaml")), configData, 0644)

	return []string{definitionsDir, configsDir}, nil
}

func MakePluginDir(version int, name, ext string, data []byte) (dir string, err error) {
	dir, err = ioutil.TempDir("", "plugindata")
	if err != nil {
		return
	}
	if version == 0 {
		err = os.MkdirAll(filepath.Join(dir, name), 0755)
		if err != nil {
			return
		}
		err = ioutil.WriteFile(filepath.Join(dir, name, "plugin.yaml"), data, 0644)
	} else {
		err = ioutil.WriteFile(filepath.Join(dir, fmt.Sprintf("%s.%s", name, ext)), data, 0644)
	}
	return
}

func (s *RegistrySuite) TestLoadPlugins(c *C) {

	configData := []byte(`
name: foo
description: foo plugin
source:
  - command:
     - cmd
     - -switch
     - arg
    prefix: my/prefix
    interval: 42
    type: inventory
os: linux, darwin, windows
property:
  - name: p1
    description: d1
  - name: p2
    description: d2
`)
	dir, err := MakePluginDir(0, "foo", "yaml", configData)
	c.Assert(err, IsNil)
	r := NewPluginRegistry([]string{dir}, []string{})
	plugin := &Plugin{
		Name:       "foo",
		workingDir: filepath.Join(dir, "foo"),
	}
	pluginDir := r.GetPluginDir(plugin)
	c.Assert(len(pluginDir), Not(Equals), 0)

	err = r.LoadPlugins()
	c.Assert(err, IsNil)

	readPlugin, err := r.GetPlugin("foo")
	c.Assert(err, IsNil)
	c.Assert(readPlugin, NotNil)
	c.Assert(readPlugin.Name, Equals, "foo")
	c.Assert(len(readPlugin.Sources), Equals, 1)
	c.Assert(len(readPlugin.Properties), Equals, 2)
	source := readPlugin.Sources[0]
	c.Assert(source.Command, DeepEquals, []string{"cmd", "-switch", "arg"})
	c.Assert(source.Interval, Equals, 42)
	c.Assert(readPlugin.Properties[0].Name, Equals, "p1")
	c.Assert(readPlugin.Properties[0].Description, Equals, "d1")
}

func (s *RegistrySuite) TestLoadV1Plugins(c *C) {

	definitionData := []byte(`
name: foo
description: foo plugin
protocol_version: 1
os: linux, darwin, windows

commands:
  one:
    command:
      - cmd
      - -switch
      - arg
    prefix: my/prefix
    interval: 42
`)

	configData := []byte(`integration_name: foo

instances:
  - name: foo
    command: one
    arguments:
      key: value
      key2: value2
`)
	dirs, err := MakePluginV1Dirs("foo", definitionData, configData)
	c.Assert(err, IsNil)
	r := NewPluginRegistry([]string{dirs[0]}, []string{dirs[1]})
	plugin := &Plugin{
		Name:       "foo",
		workingDir: dirs[0],
	}
	pluginDir := r.GetPluginDir(plugin)
	c.Assert(len(pluginDir), Not(Equals), 0)

	err = r.LoadPlugins()
	c.Assert(err, IsNil)

	readPlugin, err := r.GetPlugin("foo")
	c.Assert(err, IsNil)
	c.Assert(readPlugin, NotNil)
	c.Assert(readPlugin.Name, Equals, "foo")
	c.Assert(readPlugin.ProtocolVersion, Equals, 1)
	c.Assert(len(readPlugin.Commands), Equals, 1)

	command := readPlugin.Commands["one"]
	c.Assert(command.Command, DeepEquals, []string{"cmd", "-switch", "arg"})
	c.Assert(command.Interval, Equals, 42)
	c.Assert(command.Prefix.String(), Equals, "my/prefix")

	instance := r.GetPluginInstances()

	c.Assert(len(instance[0].Arguments), Equals, 2)
	c.Assert(instance[0].Arguments["key"], Equals, "value")
}

func (s *RegistrySuite) TestLoadV1PluginsBadCommandFormatSkipsLoading(c *C) {

	definitionData := []byte(`
name: foo
description: foo plugin
protocol_version: 1
os: linux, darwin, windows

commands:
  one:
    command: cmd
    prefix: my/prefix
    interval: 42
`)

	configData := []byte(`integration_name: foo

instances:
  - name: foo
    command: one
    arguments:
      key: value
      key2: value2
`)
	dirs, err := MakePluginV1Dirs("foo", definitionData, configData)
	c.Assert(err, IsNil)
	r := NewPluginRegistry([]string{dirs[0]}, []string{dirs[1]})

	err = r.LoadPlugins()
	c.Assert(err, IsNil)

	_, err = r.GetPlugin("foo")
	c.Assert(err, ErrorMatches, "Integration definition not found")

}

func (s *RegistrySuite) TestLoadV1PluginsDefaultPrefix(c *C) {
	definitionData := []byte(`
name: foo-name
description: foo plugin
protocol_version: 1
commands:
  one:
    command:
      - cmd
    interval: 42
`)
	configData := []byte(`
integration_name: foo-name

instances:
  - name: foo
    command: bar
    arguments:
      key: value
`)
	dirs, err := MakePluginV1Dirs("foo2", definitionData, configData)
	c.Assert(err, IsNil)
	r := NewPluginRegistry([]string{dirs[0]}, []string{dirs[1]})
	plugin := &Plugin{
		Name:       "foo-name",
		workingDir: dirs[0],
	}
	pluginDir := r.GetPluginDir(plugin)
	c.Assert(len(pluginDir), Not(Equals), 0)

	err = r.LoadPlugins()
	c.Assert(err, IsNil)

	readPlugin, err := r.GetPlugin("foo-name")
	c.Assert(err, IsNil)
	c.Assert(readPlugin, NotNil)
	c.Assert(readPlugin.Name, Equals, "foo-name")
	c.Assert(readPlugin.ProtocolVersion, Equals, 1)
	command := readPlugin.Commands["one"]
	c.Assert(command.Interval, Equals, 42)
	c.Assert(command.Prefix.String(), Equals, "integration/foo-name")
}

func (s *RegistrySuite) TestLoadV1PluginsYML(c *C) {
	definitionData := []byte(`
name: foo-name
description: foo plugin
protocol_version: 1
commands:
  one:
    command:
      - ./cmd -switch arg
    prefix: my/prefix
    interval: 42
`)

	configData := []byte(`
integration_name: foo-name

instances:
  - name: foo
    command: bar
    arguments:
      key: value

  - name: bar
    command: bar
    arguments:
      key: value2
`)
	dirs, err := MakePluginV1Dirs("foo2", definitionData, configData)
	c.Assert(err, IsNil)
	r := NewPluginRegistry([]string{dirs[0]}, []string{dirs[1]})
	plugin := &Plugin{
		Name:       "foo-name",
		workingDir: dirs[0],
	}
	pluginDir := r.GetPluginDir(plugin)
	c.Assert(len(pluginDir), Not(Equals), 0)

	err = r.LoadPlugins()
	c.Assert(err, IsNil)

	readPlugin, err := r.GetPlugin("foo-name")
	c.Assert(err, IsNil)
	c.Assert(readPlugin, NotNil)
	c.Assert(readPlugin.Name, Equals, "foo-name")
	c.Assert(readPlugin.ProtocolVersion, Equals, 1)
}

func (s *RegistrySuite) TestExamplePlugin(c *C) {
	configData := []byte(`
# Basic plugin metadata
name: example-plugin
description: An example plugin which monitors the list of files in a configured directory.

# A list of properties required by the plugin. If these properties are not specified
# in newrelic-infra-plugins.yml, the plugin will not be used.
property:
  - name: directoryToMonitor
    description: The full path to a directory to be monitored

# Data sources for the plugin. The given command and arguments will be executed
# every [interval] seconds, producing inventory data for the [prefix] path.
# Commands are invoked relative to the plugin directory, so relative paths can
# be used to access files included with the plugin.
source:
  - command:
     - python
     - ls-dir.py
     - $directoryToMonitor
    prefix: config/ls-$directoryToMonitor
    interval: 5
    type: inventory
`)
	dir, err := MakePluginDir(0, "example-plugin", "yaml", configData)
	c.Assert(err, IsNil)
	r := NewPluginRegistry([]string{dir}, []string{})

	err = r.LoadPlugins()
	c.Assert(err, IsNil)
	readPlugin, err := r.GetPlugin("example-plugin")
	c.Assert(err, IsNil)
	c.Assert(readPlugin, NotNil)
}
