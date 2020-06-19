// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"bufio"
	"io/ioutil"
	"os"
	"strings"
)

const (
	// file path without etc prefix, use host etc in init to get appropriate prefix
	DefaultLinuxOsReleasePath = "/os-release"
)

var (
	osReleaseFilePath = DefaultLinuxOsReleasePath
)

func GetOS() int {
	return OS_LINUX
}

// detect running linux platform/distro
func GetLinuxDistro() int {
	if info, err := GetLinuxOSInfo(); err == nil {
		// More Specific Tests First
		if identity, ok := info["ID"]; ok {
			switch {
			case identity == "coreos":
				return LINUX_COREOS
			case identity == "sles":
				return LINUX_SUSE
			}
		}
		// Look alikes
		if like, ok := info["ID_LIKE"]; ok {
			switch {
			case like == "debian":
				return LINUX_DEBIAN
			case strings.Contains(like, "rhel"), strings.Contains(like, "fedora"):
				return LINUX_REDHAT
			}
		}
	}
	if _, err := os.Open(HostEtc("/redhat-release")); err == nil {
		return LINUX_REDHAT
	}

	if _, err := os.Open(HostEtc("/debian_version")); err == nil {
		return LINUX_DEBIAN
	}

	if IsAmazonOS() {
		return LINUX_AWS_REDHAT
	}

	return LINUX_UNKNOWN
}

func GetLinuxOSInfo() (info map[string]string, err error) {
	osFile := HostEtc(osReleaseFilePath)
	file, err := os.Open(osFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info = make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := strings.Split(scanner.Text(), "="); len(line) == 2 {
			// strip any surrounding quotation marks
			info[line[0]] = strings.Trim(line[1], "\"")
		}
	}
	err = scanner.Err()
	return
}

// Detect whether the host is running Amazon Linux by looking for known AWS Linux OS files.
func IsAmazonOS() bool {
	if _, err := os.Open(HostEtc("/ec2_version")); err == nil {
		return true
	}
	if release, err := ioutil.ReadFile(HostEtc("/issue")); err == nil {
		if strings.Contains(string(release), "Amazon") {
			return true
		}
	}

	if _, err := os.Open(HostEtc("/yum.repos.d/amzn-main.repo")); err == nil {
		return true
	}

	return false
}
