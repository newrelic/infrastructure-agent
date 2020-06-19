// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"time"
)

// Long running integration that sends heartbeats to not be killed by the agent timeout

const heartBeatPeriod = 100 * time.Millisecond
const metricPeriod = 1 * time.Second

func main() {

	heartBeat := time.NewTicker(heartBeatPeriod)
	metric := time.NewTicker(metricPeriod)

	for {
		select {
		case <-heartBeat.C:
			fmt.Println("{}")
		case <-metric.C:
			fmt.Print(`{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":[`)
			fmt.Println(`{"event_type":"LREvent","value":"hello"}]}`)
		}
	}
}
