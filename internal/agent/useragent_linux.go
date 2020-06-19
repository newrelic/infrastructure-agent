// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func GenerateUserAgent(what, version string) string {
	debugData := make(map[string]string)

	var err error
	debugData["host"], err = os.Hostname()
	if err != nil {
		debugData["host"] = "unknown"
	}

	issue, err := ioutil.ReadFile("/etc/issue")
	if err != nil {
		debugData["os"] = "unknown"
	} else {
		// grab only the first line of /etc/issue
		scanner := bufio.NewScanner(bytes.NewBuffer(issue))
		for scanner.Scan() {
			debugData["os"] = strings.TrimSpace(scanner.Text())
			break
		}
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
