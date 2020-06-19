// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fingerprint

import "github.com/newrelic/infrastructure-agent/pkg/helpers"

func GetBootId() string {
	return helpers.ReadFirstLine(helpers.HostProc("/sys/kernel/random/boot_id"))
}
