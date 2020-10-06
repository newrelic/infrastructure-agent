// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/json"
	"strconv"
)

// Run request versions
const (
	VUnsupported = Version(0)
	V1           = Version(1)
)

type Version int

// cmdRequestDiscriminator represents the JSON shape for an integration run request
type cmdRequestDiscriminator struct {
	CommandRequestVersion string `json:"command_request_version"`
}

// {
//   "command_request_version": "1",
//   "commands": [
//     {
//       "name": "foo":,
//       "command": "/foo",
//       "args": ["-bar", "baz"],
//       "env": {
//         "FOO": "BAR",
//       }
//     }
//   ]
// }
type RunRequest struct {
}

// IsCommandRequest guesses whether a json line (coming through a previous integration response)
// belongs to command request payload, providing which version it belongs to in case it succeed.
func IsCommandRequest(line []byte) (isCmdRequest bool, cmdRequestVersion Version) {
	cmdRequestVersion = VUnsupported

	var d cmdRequestDiscriminator
	if err := json.Unmarshal(line, &d); err != nil {
		return
	}

	versionInt, err := strconv.Atoi(d.CommandRequestVersion)
	if err != nil {
		return
	}

	cmdRequestVersion = Version(versionInt)
	isCmdRequest = true
	return
}
