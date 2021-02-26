// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// Dummy integration reporting a command request.

func main() {
	projectDir, err := filepath.Abs("./")
	if err != nil {
		panic(err)
	}
	// aimed to be triggered from v4 manager_test.go
	dir := filepath.Join(projectDir, "fixtures", "cfgreq")

	fmt.Println(strings.Replace(`{
	"config_protocol_version": "1",
	"action": "register_config",
	"config_name": "myconfig",
	"config": {
		"variables": {},
		"integrations": [
			{
				"name": "nri-test",
				"exec": ["`+path.Join(dir, "v4.sh")+`"]
			}
		]
	}
	}`, "\n", "", -1))
}
