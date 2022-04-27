// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"os"
	"time"
)

// Long time running integration that submits a "first" metric, then the value passed by the first command line
// argument or the VALUE env var (or "unset" if not set).
//
// CAUTION!! this integration does not respect any interval limit and continuously sends data

func main() {
	secondValue := "unset"
	if len(os.Args) > 1 {
		secondValue = os.Args[1]
	} else if sval, ok := os.LookupEnv("VALUE"); ok {
		secondValue = sval
	}
	value := "first"

	for {
		fmt.Print(`{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":[`)
		fmt.Printf(`{"event_type":"LTEvent","value":"%s"}`, value)
		fmt.Println("]}")
		value = secondValue
		time.Sleep(10 * time.Millisecond)
	}
}
