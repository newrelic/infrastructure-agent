// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

// OnDemand can be used during Replace and ReplaceBytes to dynamically get variable values given a variable name (key).
type OnDemand func(key string) (value []byte, found bool)

// Provided configures the Replace and ReplaceBytes functions to get variables from the OnDemand function.
// If a given variable is not found on the variables/discovery replacement, the variable name is first
// looked up in the OnDemand provider before being replaced or discarded as not found.
func Provided(od OnDemand) ReplaceOption {
	return func(rc *replaceConfig) {
		rc.onDemand = append(rc.onDemand, od)
	}
}
