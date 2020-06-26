// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discovery

import "github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"

type Discovery struct {
	Variables         data.Map             `json:"variables"`
	MetricAnnotations data.InterfaceMap    `json:"annotations"`
	EntityRewrites    []data.EntityRewrite `json:"entityRewrites"`
}
