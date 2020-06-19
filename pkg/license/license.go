// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package license

import "regexp"

var (
	// We get our license from APM's Agent Key generator
	licenseRegex       = regexp.MustCompile("^[[:alnum:]]+$")
	regionLicenseRegex = regexp.MustCompile(`^([a-z]{2,3}[0-9]{2})x{1,2}`)
)

// IsValid return true if license is in valid format.
func IsValid(licenseKey string) bool {
	return licenseRegex.MatchString(licenseKey)
}

// IsRegionEU returns true if license region is EU.
func IsRegionEU(license string) bool {
	r := GetRegion(license)
	// only EU supported
	if len(r) > 1 && r[:2] == "eu" {
		return true
	}
	return false
}

// GetRegion returns license region or empty if none.
func GetRegion(licenseKey string) string {
	matches := regionLicenseRegex.FindStringSubmatch(licenseKey)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}
