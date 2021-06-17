// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Dummy integration reporting a config request.

func main() {
	stdoutType := "v4"
	if val, ok := os.LookupEnv("STDOUT_TYPE"); ok {
		stdoutType = val
	}
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
				"name": "spawned_integration",
				"exec": ["`+path.Join(dir, "spawned_integration.sh")+`"],
				"env": {
					"STDOUT_TYPE": "`+stdoutType+`"
				} 
			}
		]
	}
	}`, "\n", "", -1))
}
