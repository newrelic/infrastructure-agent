//go:build integration
// +build integration

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

func main() {
	path := flag.String("path", "", "")
	multiLine := flag.Bool("multiLine", false, "")
	times := flag.Int("times", 100, "")
	sleepTime := flag.Duration("sleepTime", 2*time.Second, "")
	mode := flag.String("mode", "short", "")
	configPath := os.Getenv("CONFIG_PATH")
	flag.String("nri-process-name", "unknown", "")
	flag.Parse()

	// If config file is present it replace the configs from flag for testing pourposes.
	if configPath != "" {
		cfgFile, err := ioutil.ReadFile(configPath)
		if err != nil {
			panic("fail to read config file!")
		}
		var m map[string]interface{}
		if err := yaml.Unmarshal(cfgFile, &m); err != nil {
			panic("fail to unmarshal config file!")
		}

		if p, ok := m["path"]; ok {
			*path = fmt.Sprintf("%v", p)
		}
	}

	content, err := ioutil.ReadFile(*path)
	if err != nil {
		panic(err)
	}
	contentStr := string(content)
	if !*multiLine {
		contentStr = strings.ReplaceAll(contentStr, "\n", "")
	}
	switch strings.ToLower(*mode) {
	case "long":
		for i := 0; i < *times; i++ {
			fmt.Println(contentStr)
			time.Sleep(*sleepTime)
		}
	case "short":
		fmt.Println(contentStr)
		time.Sleep(*sleepTime)
	default:
		panic("unsupported running mode!")
	}

}
