// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package distro

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/newrelic/infrastructure-agent/internal/os/fs"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var r = regexp.MustCompile(`^(Red Hat|CentOS).* release 5\..*$`)

func GetDistro() string {
	if info, err := helpers.GetLinuxOSInfo(); err == nil {
		if name, ok := info["PRETTY_NAME"]; ok {
			return strings.TrimSpace(name)
		}
		if name, ok := info["NAME"]; ok {
			return strings.TrimSpace(fmt.Sprintf("%s %s", name, info["VERSION"]))
		}
	}

	platform := helpers.GetLinuxDistro()
	switch {
	case platform == helpers.LINUX_DEBIAN:
		distroRe := regexp.MustCompile(`DISTRIB_DESCRIPTION="(.*?)"`)
		description, _ := fs.ReadFileFieldMatching(helpers.HostEtc("/lsb_release"), distroRe)
		if len(description) > 0 {
			return strings.TrimSpace(description)
		}
		return "Debian (lsb_release not available)"
	case platform == helpers.LINUX_REDHAT:
		line, err := fs.ReadFirstLine(helpers.HostEtc("/redhat-release"))
		if err == nil {
			return line
		}
	}
	return "unknown"
}

func IsCentos5() bool {
	return r.MatchString(GetDistro())
}
