// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Shared test helpers
package integrationtest

import (
	"errors"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
)

var ErrLookup = integration.InstancesLookup{
	Legacy: func(_ integration.DefinitionCommandConfig) (integration.Definition, error) {
		return integration.Definition{}, errors.New("legacy integrations provider not expected to be invoked")
	},
	ByName: func(_ string) (string, error) {
		return "", errors.New("lookup by name not expected to be invoked")
	},
}
