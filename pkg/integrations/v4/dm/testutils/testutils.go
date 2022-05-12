// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
)

// NoopEmitter /dev/null sink.
type NoopEmitter struct{}

func NewNoopEmitter() dm.Emitter {
	return &NoopEmitter{}
}

func (e *NoopEmitter) Send(_ fwrequest.FwRequest) {}

// RecordEmitter stores all received requests.
type RecordEmitter struct {
	received []fwrequest.FwRequest
}

// implementation fulfills the interface.
var _ dm.Emitter = &RecordEmitter{}

func NewRecordEmitter() *RecordEmitter {
	return &RecordEmitter{
		received: []fwrequest.FwRequest{},
	}
}

func (e *RecordEmitter) Send(r fwrequest.FwRequest) {
	e.received = append(e.received, r)
}

func (e *RecordEmitter) Received() []fwrequest.FwRequest {
	return e.received
}
