// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// Integration fixture that validates a YAML file passed as argument. It expects a YAML in the following form:
type expectedYAML struct {
	EventType string            `yaml:"event_type" json:"event_type"`
	Map       map[string]string `yaml:"map" json:"map"`
	Array     []string          `yaml:"array" json:"array"`
}

// If the YAML is wrong, exits with non-zero status and logs a message in the standard error. If the YAML is
// correct, it emits the received YAML as JSON
func main() {
	filePath, ok := os.LookupEnv("CONFIG_PATH")
	if !ok {
		if len(os.Args) < 2 {
			_, _ = fmt.Fprintln(os.Stderr, "expecting YAML file as an argument")
			os.Exit(-1)
		}
		filePath = os.Args[1]
	}
	yamlBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error reading YAML file: %s\n", err.Error())
		os.Exit(-1)
	}
	var yamlContent expectedYAML
	if err := yaml.Unmarshal(yamlBytes, &yamlContent); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error parsing YAML file: %s\n", err.Error())
		os.Exit(-1)
	}
	yamlJson, err := json.Marshal(yamlContent)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error converting YAML to JSON: %s\n", err.Error())
		os.Exit(-1)
	}

	// Everything OK! returning the input YAML as an integration JSON
	fmt.Print(`{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":[`)
	fmt.Print(string(yamlJson))
	fmt.Println("]}")
}
