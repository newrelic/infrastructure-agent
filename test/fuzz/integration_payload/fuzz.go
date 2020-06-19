// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build gofuzz
// Fuzz testing via https://github.com/dvyukov/go-fuzz

package integration_payload

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
)

// Several funcs could be used but they should be passed to the go-fuzz cmd
// Therefore a good approach to cover different entry points could be to place a single Fuzz func
// per entry point, running on their own corpuses, each one in its respective entry point folder.
// Ref: https://github.com/dvyukov/go-fuzz/issues/60

// Fuzz tests integration payload handling.
func Fuzz(data []byte) int {
	// integration protocol <= v4
	_, _, err1 := legacy.ParsePayload(data, true)
	_, _, err2 := legacy.ParsePayload(data, false)

	// integration protocol v4
	// otherwise parse won't happen
	ffm := feature_flags.NewManager(map[string]bool{handler.ProtocolV4Enabled: true})
	_, err3 := emitter.ParsePayloadV4(data, ffm)

	// discourage mutation when no errors at all
	if err1 == nil && err2 == nil && err3 == nil {
		return -1
	}

	return 0
}
