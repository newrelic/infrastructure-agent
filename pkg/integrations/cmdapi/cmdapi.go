// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmdapi

var (
	integrationsAllowedToRunStopFromCmdAPI = map[string]struct{}{
		"nri-lsi-java":          {},
		"nri-process-discovery": {},
	}
)

func IsForbiddenToRunStopFromCmdAPI(integrationName string) bool {
	_, ok := integrationsAllowedToRunStopFromCmdAPI[integrationName]

	return !ok
}
