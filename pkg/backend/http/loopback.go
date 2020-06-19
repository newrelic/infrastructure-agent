// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"regexp"
	"strings"
)

// For IPV6 we check for:
// - Strings starting with `::1:`
// - That `::1` appears between two attributes separated by `:` like `service:::1:80`
// - That the value is exactly `::1`
var localhostRE = regexp.MustCompile(`(localhost|LOCALHOST|127(?:\.[0-9]+){0,2}\.[0-9]+|^::1$|:::1:|^::1:)`)

// A different regex is needed for replacing because `localhostRE` matches
// IPV6 by using extra `:` that don't belong to the IP but are separators.
var localhostReplaceRE = regexp.MustCompile(`(localhost|LOCALHOST|127(?:\.[0-9]+){0,2}\.[0-9]+|::1)`)

const (
	LOCALHOST           = "localhost"
	LOCALHOST_IPV6      = "::1"
	LOCALHOST_PREFIX_IP = "127."
)

// ContainsLocalhost checks if the given value contains a reference a
// localhost hostname or ip
func ContainsLocalhost(value string) bool {
	value = strings.ToLower(value)
	for _, lh := range []string{LOCALHOST, LOCALHOST_PREFIX_IP, LOCALHOST_IPV6} {
		//we use strings.Contains for performance reasons in order to avoid regex operations for non localhost strings.
		if strings.Contains(value, lh) {
			return localhostRE.Match([]byte(value))
		}
	}

	return false
}

// IsLocalhost checks if the given value is equal to a localhost hostname
// or ip
func IsLocalhost(value string) bool {
	return strings.ToLower(value) == LOCALHOST || value == LOCALHOST_IPV6 || strings.HasPrefix(value, LOCALHOST_PREFIX_IP)
}

// ReplaceLocalhost replaces the occurrence of a localhost address with
// the given hostname
func ReplaceLocalhost(source, with string) string {
	return localhostReplaceRE.ReplaceAllString(source, with)
}
