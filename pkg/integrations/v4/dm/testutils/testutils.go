// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
)

type NoopEmitter struct{}

func (e *NoopEmitter) Send(_ fwrequest.FwRequest) {}

func NewNoopEmitter() dm.Emitter {
	return &NoopEmitter{}
}
