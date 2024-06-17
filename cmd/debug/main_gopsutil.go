package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

var matcher = func(interface{}) bool { return true }

func cpu_sample(l *log.Logger, interval *time.Duration) {
	dataDir, err := ioutil.TempDir("", "prefix")
	if err != nil {
		panic(err)
	}

	cfg := config.NewTest(dataDir)

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	lookups := agent.NewIdLookup(hostname.CreateResolver("", "", true), cloudDetector, cfg.DisplayName)

	ctx := agent.NewContext(cfg, "1.2.3", testhelpers.NullHostnameResolver, lookups, matcher)

	cpuMonitor := metrics.NewCPUMonitor(ctx)

	// cpuSample, err := cpuMonitor.Sample()
	// if err != nil {
	// 	panic(err)
	// }

	headers := []string{
		"In Use", "User", "System",
		//  "IOWait",
		"Idle",
		//  "Steal"
	}
	formatterStr := "| %-10s"
	formatterNum := "| %-10.4f"
	var resH string
	var ticker int

	for _, v := range headers {
		resH += fmt.Sprintf(formatterStr, v)
	}

	// fmt.Println(resH)

	for {
		cpuSample, err := cpuMonitor.Sample()
		if err != nil {
			panic(err)
		}
		// marshaled, err := json.Marshal(cpuSample)
		// if err != nil {
		// 	panic(err)
		// }
		// fmt.Println(string(marshaled))

		if ticker%100 == 0 {
			Log(l, resH)
		}

		var resN string
		resN += fmt.Sprintf(formatterNum, cpuSample.CPUPercent)
		resN += fmt.Sprintf(formatterNum, cpuSample.CPUUserPercent)
		resN += fmt.Sprintf(formatterNum, cpuSample.CPUSystemPercent)
		// resN += fmt.Sprintf(formatterNum, cpuSample.CPUIOWaitPercent)
		resN += fmt.Sprintf(formatterNum, cpuSample.CPUIdlePercent)
		// resN += fmt.Sprintf(formatterNum, cpuSample.CPUStealPercent)

		// Log(l, resN)

		time.Sleep(*interval)
		ticker++
	}

}
