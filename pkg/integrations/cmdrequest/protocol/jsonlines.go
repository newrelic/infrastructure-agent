// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/json"
	"strconv"
)

// Command request versions:
const (
	VUnsupported = Version(0)
	V1           = Version(1)
)

type Version int

// CmdRequestDiscriminator represents the JSON shape for an integration command request.
type CmdRequestDiscriminator struct {
	CommandRequestVersion string `json:"command_request_version"`
}

// CmdRequestV1 carries an integration payload requesting command/s execution.
// This applies to a command request v1.
// Payload comes from JSON decoding. Expected shape is:
//
//	{
//	  "command_request_version": "1",
//	  "commands": [
//	    {
//	      "name": "foo":,
//	      "command": "/foo",
//	      "args": ["-bar", "baz"],
//	      "env": {
//	        "FOO": "BAR",
//	      }
//	    }
//	  ]
//	}
type CmdRequestV1 struct {
	CmdRequestDiscriminator
	Commands []CmdRequestV1Cmd `json:"commands"`
}

type CmdRequestV1Cmd struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// IsCommandRequest guesses whether a json line (coming through a previous integration response)
// belongs to command request payload, providing which version it belongs to in case it succeed.
func IsCommandRequest(line []byte) (isCmdRequest bool, cmdRequestVersion Version) {
	cmdRequestVersion = VUnsupported

	var d CmdRequestDiscriminator
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

func DeserializeLine(line []byte) (r CmdRequestV1, err error) {
	err = json.Unmarshal(line, &r)
	return
}
