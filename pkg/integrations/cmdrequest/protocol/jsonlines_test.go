// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fixture struct {
	Payload  string
	ParsedV1 CmdRequestV1
}

// Test fixtures
var (
	fixtureFoo = fixture{
		Payload: `
{
  "command_request_version": "1",
  "commands": [
    {
      "name": "foo",
      "command": "/foo",
      "args": ["-bar", "baz"],
      "env": {
        "FOO": "BAR"
      }
    }
  ]
}`,
		ParsedV1: CmdRequestV1{
			CmdRequestDiscriminator{CommandRequestVersion: "1"},
			[]CmdRequestV1Cmd{
				{
					Name:    "foo",
					Command: "/foo",
					Args:    []string{"-bar", "baz"},
					Env: map[string]string{
						"FOO": "BAR",
					},
				},
			},
		},
	}

	fixtureWithNullValues = fixture{
		Payload: `
{
  "command_request_version": "1",
  "commands": [
    {
      "name": "foo",
      "command": "/foo",
      "args": null,
      "env": null
    }
  ]
}`,
		ParsedV1: CmdRequestV1{
			CmdRequestDiscriminator{CommandRequestVersion: "1"},
			[]CmdRequestV1Cmd{
				{
					Name:    "foo",
					Command: "/foo",
				},
			},
		},
	}
)

func inline(content string) string {
	return strings.Replace(content, "\n", "", -1)
}

func TestIsCommandRequest(t *testing.T) {
	tests := []struct {
		name                  string
		line                  string
		wantIsCmdRequest      bool
		wantCmdRequestVersion Version
	}{
		{
			"empty case",
			"",
			false,
			VUnsupported,
		},
		{
			"garbage",
			"\n\r",
			false,
			VUnsupported,
		},
		{
			"simple case",
			inline(fixtureFoo.Payload),
			true,
			V1,
		},
		{
			"version value should be string, not number",
			inline(`{
  "command_request_version": 1,
  "commands": []
}`),
			false,
			VUnsupported,
		},
		{
			"wrong command content doesn't affect determining it's a cmd request and version",
			// still well formatted JSON is required
			inline(`{
  "command_request_version": "1",
  "commands": [
    {
      "this is": "wrong"
    }
  ]
}`),
			true,
			V1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIsCmdRequest, gotCmdRequestVersion := IsCommandRequest([]byte(tt.line))
			assert.Equal(t, tt.wantIsCmdRequest, gotIsCmdRequest)
			assert.Equal(t, tt.wantCmdRequestVersion, gotCmdRequestVersion)
		})
	}
}

func TestUnmarshall(t *testing.T) {
	r, err := DeserializeLine([]byte(inline(fixtureFoo.Payload)))
	assert.NoError(t, err)

	assert.Equal(t, fixtureFoo.ParsedV1, r)
}

func TestUnmarshallWithNullValues(t *testing.T) {
	r, err := DeserializeLine([]byte(inline(fixtureWithNullValues.Payload)))
	assert.NoError(t, err)

	assert.Equal(t, fixtureWithNullValues.ParsedV1, r)
}
