// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/pkg/errors"
)

// Protocol Integration versions.
const (
	V1 = 1
	V2 = 2
	V3 = 3
	V4 = 4
)

var (
	IntProtocolNotSupportedErr = errors.New("integration protocol version not supported")
)

// VersionFromPayload determines the protocol version number from integration payload
// for both inventory and metrics.
func VersionFromPayload(raw []byte, forceV2ToV3Upgrade bool) (protocolVersion int, err error) {
	if len(raw) == 0 {
		err = errors.New("no content to parse")
		return
	}

	var payloadProtocolVersion PluginProtocolVersion
	if err = json.Unmarshal(raw, &payloadProtocolVersion); err != nil {
		return
	}

	protocolVersion, err = versionFromParsed(payloadProtocolVersion, forceV2ToV3Upgrade)

	if err == nil && protocolVersion > V4 {
		err = IntProtocolNotSupportedErr
	}

	return
}

// Version returns the protocol version or error if the given raw protocol is invalid.
// In case we force protocol V3, will return v3 for both V2 and V3 protocols
func versionFromParsed(payloadProtocolVersion PluginProtocolVersion, forceV2ToV3Upgrade bool) (int, error) {
	if payloadProtocolVersion.RawProtocolVersion == nil {
		return 0, errors.New("protocol_version is not defined")
	}

	var protocolV int
	switch typedVersion := payloadProtocolVersion.RawProtocolVersion.(type) {
	case string:
		parsedVersion, err := strconv.ParseInt(typedVersion, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("Protocol version '%v' could not be parsed as an integer.", typedVersion)
		}
		protocolV = int(parsedVersion)
	case float64:
		if math.Remainder(typedVersion, 1) != float64(0) {
			return 0, fmt.Errorf("Protocol version %v was a float, not an integer.", typedVersion)
		}
		protocolV = int(typedVersion)
	default:
		return 0, fmt.Errorf("Protocol version '%v' could not be parsed as an integer.", typedVersion)
	}

	if protocolV > V4 || protocolV < V1 {
		return 0, fmt.Errorf("unsupported protocol version: %v. Please try updating the Agent to the newest version.", protocolV)
	}

	if protocolV == V2 && forceV2ToV3Upgrade {
		return V3, nil
	}

	return protocolV, nil
}
