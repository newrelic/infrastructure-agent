package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

var matcher = func(interface{}) bool { return true }

func main() {
	interval := flag.Duration("interval", 1*time.Second, "Interval between samples")
	flag.Parse()

	dataDir, err := ioutil.TempDir("", "prefix")
	if err != nil {
		panic(err)
	}

	cfg := config.NewTest(dataDir)

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	lookups := agent.NewIdLookup(hostname.CreateResolver("", "", true), cloudDetector, cfg.DisplayName)

	ctx := agent.NewContext(cfg, "1.2.3", testhelpers.NullHostnameResolver, lookups, matcher)

	cpuMonitor := metrics.NewCPUMonitor(ctx)

	cpuSample, err := cpuMonitor.Sample()
	if err != nil {
		panic(err)
	}

	for {
		cpuSample, err = cpuMonitor.Sample()
		if err != nil {
			panic(err)
		}
		marshaled, err := json.Marshal(cpuSample)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(marshaled))
		time.Sleep(*interval)
	}

}
