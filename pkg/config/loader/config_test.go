// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config_loader

import (
	"io/ioutil"
	"os"

	. "gopkg.in/check.v1"
)

type ConfigSuite struct{}

var _ = Suite(&ConfigSuite{})

func CreateTestFile(data []byte) (*os.File, error) {
	tmp, err := ioutil.TempFile("", "loadconfig")
	if err != nil {
		return nil, err
	}
	_, err = tmp.Write(data)
	if err != nil {
		return nil, err
	}
	tmp.Close()
	return tmp, nil
}

func (self *ConfigSuite) TestLoadYamlConfig(c *C) {
	yamlData := []byte(`param: hello`)

	tmp, err := CreateTestFile(yamlData)
	c.Assert(err, IsNil)
	defer os.Remove(tmp.Name())

	var config struct {
		Param string `yaml:"param"`
	}

	meta, err := LoadYamlConfig(&config, tmp.Name())
	c.Assert(err, IsNil)
	c.Assert(config.Param, Equals, "hello")
	c.Assert(meta, NotNil)
	c.Assert(meta.Contains("param"), Equals, true)
	c.Assert(meta.Contains("otherParam"), Equals, false)
}

func (self *ConfigSuite) TestMissingLoadYamlConfig(c *C) {

	config := &struct {
		Param string `yaml:"param"`
	}{}

	meta, err := LoadYamlConfig(&config, "something.yml")
	c.Assert(err, IsNil)
	c.Assert(config.Param, Equals, "")
	c.Assert(meta, NotNil)
	c.Assert(meta.Contains("param"), Equals, false)
	c.Assert(meta.Contains("otherParam"), Equals, false)
}
