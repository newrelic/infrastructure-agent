// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package distro

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"regexp"
)

var (
	productNameRegex    = regexp.MustCompile("ProductName:\\s*(.*)")
	productVersionRegex = regexp.MustCompile("ProductVersion:\\s*(.*)")

	defaultProductName = "macOS"
)

type distroFetcherFn func() (string, error)

func fetchDistro() (string, error) {
	return helpers.RunCommand("/usr/bin/sw_vers", "")
}

// GetDistro will parse the output for '/usr/bin/sw_vers' command to detect
// the ProductName and ProductVersion fields
func GetDistro() string {
	return parseDistro(fetchDistro)
}

func parseDistro(distro distroFetcherFn) string {
	var productName, productVersion string
	output, err := distro()
	if err != nil {
		return defaultProductName
	}

	if productNameLine := productNameRegex.FindStringSubmatch(output); len(productNameLine) > 1 {
		productName = productNameLine[1]
	} else {
		productName = defaultProductName
	}

	if productVersionLine := productVersionRegex.FindStringSubmatch(output); len(productVersionLine) > 1 {
		productVersion = productVersionLine[1]
	} else {
		productVersion = "(Unknown)"
	}

	return fmt.Sprintf("%s %s", productName, productVersion)
}

func IsCentos5() bool {
	return false
}
