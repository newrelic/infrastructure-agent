// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package config_loader handles loading of configuration from files for services
package config_loader

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/pkg/config/envvar"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"gopkg.in/yaml.v3"
)

var clog = log.WithComponent("Configuration loader")

// YAMLMetadata stores keeps track of the keys that have been defined in a YAML.
type YAMLMetadata map[string]bool

// Contains returns true if the argument key is present in the YAMLMetadata set.
func (p YAMLMetadata) Contains(key string) bool {
	_, ok := p[key]
	return ok
}

// LoadYamlConfig will populate the given configObject (should be a pointer to a struct)
// with whichever of the given filenames it finds first. There will be no error if a
// config file is not found - the configObject is assumed to have reasonable defaults.
func LoadYamlConfig(configObject interface{}, configFilePaths ...string) error {

	for _, filePath := range configFilePaths {
		if fileExists(filePath) {
			absPath, _ := filepath.Abs(filePath)
			clog.Debugf("loading configuration from %s to hydrate %T", absPath, configObject)
			fd, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer fd.Close()

			rawConfig, err := ioutil.ReadAll(fd)
			if err != nil {
				return err
			}

			rawConfig, err = envvar.ExpandInContent(rawConfig)
			if err != nil {
				return nil
			}

			return ParseConfig(rawConfig, configObject)
		}
	}
	return nil
}

func ParseConfig(rawConfig []byte, configObject interface{}) (err error) {
	// First we unmarshall as the configuration object
	err = yaml.Unmarshal(rawConfig, configObject)
	if err != nil {
		return
	}

	return
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
