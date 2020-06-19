// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"encoding/json"
	"fmt"
	"os"
)

func GenerateUserAgent(what, version string) string {
	debugData := map[string]string{
		"os": "MacOS X",
	}

	var err error
	debugData["host"], err = os.Hostname()
	if err != nil {
		debugData["host"] = "unknown"
	}

	var debugDataStr string
	buf, err := json.Marshal(debugData)
	if err != nil {
		debugDataStr = "{}"
	} else {
		debugDataStr = string(buf)
	}

	return fmt.Sprintf("%s version %s %s", what, version, debugDataStr)
}
