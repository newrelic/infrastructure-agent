// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package config_loader handles loading of configuration from files for services
package config_loader

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

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
func LoadYamlConfig(configObject interface{}, configFilePaths ...string) (*YAMLMetadata, error) {
	var keys YAMLMetadata

	for _, filePath := range configFilePaths {
		if fileExists(filePath) {
			fd, err := os.Open(filePath)
			if err != nil {
				return nil, err
			}
			defer fd.Close()

			rawConfig, err := ioutil.ReadAll(fd)
			if err != nil {
				return nil, err
			}

			rawConfig, err = ExpandEnvVars(rawConfig)
			if err != nil {
				return nil, err
			}

			return ParseConfig(rawConfig, configObject)
		}
	}
	return &keys, nil
}

func ParseConfig(rawConfig []byte, configObject interface{}) (keys *YAMLMetadata, err error) {
	// First we unmarshall as the configuration object
	err = yaml.Unmarshal(rawConfig, configObject)
	if err != nil {
		return
	}

	// then we unmarshall as a MapSlice to get information about the present keys
	metadata := yaml.MapSlice{}
	err = yaml.Unmarshal(rawConfig, &metadata)
	if err != nil {
		return
	}

	k := YAMLMetadata{}
	for _, item := range metadata {
		k[item.Key.(string)] = true
	}
	keys = &k

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

func ExpandEnvVars(content []byte) ([]byte, error) {
	r := regexp.MustCompile(`({{ *\w+.*?}})`)
	matches := r.FindAllIndex(content, -1)

	if len(matches) == 0 {
		return content, nil
	}

	var newContent []byte
	var lastReplacement int
	for _, idx := range matches {
		evStart := idx[0] + 2 // drop {{
		evEnd := idx[1] - 2   // drop }}
		if len(content) < evStart || len(content) < evEnd {
			return content, fmt.Errorf("cannot replace configuration environment variables")
		}

		evName := strings.TrimSpace(string(content[evStart:evEnd]))
		if evVal, exist := os.LookupEnv(evName); exist {
			// quote non numerics
			if _, err := strconv.ParseFloat(evVal, 64); err != nil {
				evVal = fmt.Sprintf(`"%s"`, evVal)
			}
			newContent = append(newContent, content[lastReplacement:idx[0]]...)
			newContent = append(newContent, []byte(evVal)...)
			lastReplacement = idx[1]
		} else {
			return nil, fmt.Errorf("cannot replace configuration environment variables, missing env-var: %s", evName)
		}
	}

	if lastReplacement != len(content) {
		newContent = append(newContent, content[lastReplacement:]...)
	}

	return newContent, nil
}
