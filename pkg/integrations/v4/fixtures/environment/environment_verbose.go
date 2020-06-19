// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type response struct {
	Name               string  `json:"name"`
	ProtocolVersion    string  `json:"protocol_version"`
	IntegrationVersion string  `json:"integration_version"`
	Metrics            []event `json:"metrics"`
}

type event struct {
	EventType string `json:"event_type"`
	Value     string `json:"value"`
}

func main() {

	var envVariablesParsed []event

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		envVariablesParsed = append(envVariablesParsed, event{pair[0], fmt.Sprintf("%v", pair[1])})
	}

	responseString, _ := json.Marshal(response{
		Name:               "com.newrelic.test",
		ProtocolVersion:    "1",
		IntegrationVersion: "1.0.0",
		Metrics:            envVariablesParsed,
	})

	os.Stdout.WriteString(string(responseString))
	os.Exit(1)
}
