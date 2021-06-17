// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package event

import "strings"

// AttributesPrefix preffix used to prefix attributes with.
const AttributesPrefix = "attr."

// reservedFields reserved event keys, in lowercase for case insensitive comparison.
var reservedFields = map[string]struct{}{
	"":                         {},
	"timestamp":                {},
	"eventytype":               {},
	"entityid":                 {},
	"entityguid":               {},
	"entitykey":                {},
	"entityname":               {},
	"hostname":                 {},
	"fullhostname":             {},
	"displayname":              {},
	"agentname":                {},
	"corecount":                {},
	"agentversion":             {},
	"kernelversion":            {},
	"operatingsystem":          {},
	"windowsplatform":          {},
	"windowsfamily":            {},
	"windowsversion":           {},
	"instancetype":             {},
	"system/ram":               {},
	"transformmemorytobytes":   {},
	"processorcount":           {},
	"installedmemorymegabytes": {},
	"awsregion":                {},
	"regionname":               {},
	"zone":                     {},
	"regionid":                 {},
}

// IsReserved returns true when field name is a reserved key.
func IsReserved(field string) bool {
	prefixLen := len(AttributesPrefix)
	if len(field) > prefixLen && field[:prefixLen] == AttributesPrefix {
		return true
	}

	_, ok := reservedFields[strings.ToLower(field)]
	return ok
}
