package protocoltest

import "strings"

const FixtureFoo = `{
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
}`

const FixtureWrongIntegerVersion = `{
  "command_request_version": 1,
  "commands": []
}`

const FixtureWrongCommandShape = `{
  "command_request_version": "1",
  "commands": [
    {
      "this is": "wrong"
    }
  ]
}`

func Inline(content string) string {
	return strings.ReplaceAll(content, "\n", "")
}
