// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"os"
)

// Very simple integration that just reports once a value passed as argument or as
// environment variable

func main() {
	value := os.Getenv("VALUE")
	if value == "" {
		if len(os.Args) > 1 {
			value = os.Args[1]
		} else {
			value = "unset"
		}
	}
	fmt.Println(`{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":` +
		`[{"event_type":"TestSample","value":"` + value + `"}]}`)
}
