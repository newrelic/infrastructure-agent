// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package opamp

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/newrelic/infrastructure-agent/pkg/config"
)

func equalConfs(cnf1 *config.Config, cnf2 *config.Config) bool {
	return confToStr(cnf1) == confToStr(cnf2)
}

func confToStr(cnf interface{}) string {
	b, _ := yaml.Marshal(cnf)
	return string(b)
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		// TODO handle error
	}
	return true
}

func isEmpty(filename string) bool {
	content, err := os.ReadFile(filename)
	if err != nil {
		return true // TODO WIP assumption
	}
	return strings.TrimSpace(string(content)) == ""
}
