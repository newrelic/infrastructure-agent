// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package fixtures

const CmdExtension = ""

const LongtimeDefinition = `---
name: com.newrelic.longtime
description: Testing fixture for backwards v3 plugin compatibility
protocol_version: I don't really care. Plugins v4 ignores this
os: I don't really care

commands:
  hello:
    command:
      - ./bin/longtime
      - hello
    interval: 15
  use_env:
    command:
      - ./bin/longtime
    interval: 15
`
