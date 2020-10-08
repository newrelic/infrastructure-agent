// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"strings"
)

// Dummy integration reporting protocol v4.

func main() {
	fmt.Println(strings.Replace(`{
  "command_request_version": "1",
  "commands": [
    {
      "name": "foo",
      "command": "/foo",
      "args": ["-bar", "baz"],
      "env": {
        "FOO": "BAR"
      }
    }
  ]
}`, "\n", "", -1))
}
