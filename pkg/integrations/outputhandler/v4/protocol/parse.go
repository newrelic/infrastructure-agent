// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Errors
var (
	EmptyPayloadErr = errors.New("cannot parse empty integration payload")
)

// ParsePayload parses a JSON payload using the integration  protocol format.
// Used for all metrics (events) and inventory.
func ParsePayload(raw []byte, protocolVersion int) (dataV3 PluginDataV3, err error) {
	if len(raw) == 0 {
		err = EmptyPayloadErr
		return
	}

	if protocolVersion == V1 {
		var dataV1 PluginDataV1
		if err = json.Unmarshal(raw, &dataV1); err != nil {
			return
		}
		dataV1.convertToV3(&dataV3)
		return
	}
	err = json.Unmarshal(raw, &dataV3)

	return
}
