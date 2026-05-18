// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package license

import (
	"regexp"
	"strings"
)

var (
	// We get our license from APM's Agent Key generator
	licenseRegex       = regexp.MustCompile("^[[:alnum:]]+$")
	regionLicenseRegex = regexp.MustCompile(`^([a-z]{2,3})`)
	regionKeyRegex     = regexp.MustCompile(`^.+?x`)
)

// IsValid return true if license is in valid format.
func IsValid(licenseKey string) bool {
	return licenseRegex.MatchString(licenseKey)
}

// IsRegionEU returns true if license region is EU.
func IsRegionEU(license string) bool {
	r := GetRegion(license)
	if len(r) > 1 && r[:2] == "eu" {
		return true
	}

	return false
}

// IsFederalCompliance returns true if license is from federal customer.
func IsFederalCompliance(licenseKey string) bool {
	r := GetRegion(licenseKey)

	if r == "gov" {
		return true
	}

	return false
}

// GetRegionForURL returns the region for constructing region-aware URLs.
// Returns empty for US (default) or federal compliance licenses.
// Following Protocol 15+, only region-aware keys contain 'x' (not a valid hex digit),
// so we use its presence as the discriminator.
func GetRegionForURL(licenseKey string) string {
	if !strings.ContainsRune(licenseKey, 'x') || IsFederalCompliance(licenseKey) {
		return ""
	}

	return GetRegionPrefix(licenseKey)
}

// GetRegionPrefix extracts the region prefix by applying the ^.+?x regex then stripping
// all trailing 'x' characters. For example: "jpxx..." → "jpx" → "jp".
func GetRegionPrefix(licenseKey string) string {
	match := regionKeyRegex.FindString(licenseKey)
	if match == "" {
		return ""
	}

	return strings.TrimRight(match, "x")
}

// IsRegionJP returns true if license region is JP.
func IsRegionJP(licenseKey string) bool {
	return strings.HasPrefix(GetRegionPrefix(licenseKey), "jp")
}

// GetRegion returns license region or empty if none.
func GetRegion(licenseKey string) string {
	matches := regionLicenseRegex.FindStringSubmatch(licenseKey)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}
