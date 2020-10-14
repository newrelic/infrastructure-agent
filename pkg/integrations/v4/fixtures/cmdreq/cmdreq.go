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
	dir := filepath.Join(projectDir, "fixtures", "cmdreq")

	fmt.Println(strings.Replace(`{
  "command_request_version": "1",
  "commands": [
    {
      "name": "cmd-req-name",
      "command": "`+path.Join(dir, "v4.sh")+`",
      "args": [],
      "env": {
        "FOO": "BAR"
      }
    }
  ]
}`, "\n", "", -1))
}
