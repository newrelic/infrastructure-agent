// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package when

import "os"

// Condition is any function that can return true or false
type Condition func() bool

// FileExists creates a Condition returning true when the passed file path exists
func FileExists(path string) Condition {
	return func() bool {
		st, err := os.Stat(path)
		if err != nil {
			return false
		}
		return !st.IsDir()
	}
}

// EnvExists creates a Condition returning true when the environment variables
// are defined in the environment and matched the values passed as arguments.
func EnvExists(envVars map[string]string) Condition {
	return func() bool {
		for k, v := range envVars {
			foundValue, found := os.LookupEnv(k)
			if !found || foundValue != v {
				return false
			}
		}
		return true
	}
}

// All returns true if and only if all the passed conditions are true.
// If an empty conditions list is passed, it also returns true.
func All(conditions ...Condition) bool {
	for _, cond := range conditions {
		if !cond() {
			return false
		}
	}
	return true
}
