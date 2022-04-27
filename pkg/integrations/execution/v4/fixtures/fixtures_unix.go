// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package fixtures

import (
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/testhelp"
)

var (
	IntegrationScript        = testhelp.Script("../fixtures/integration.sh")
	IntegrationVerboseScript = testhelp.Script("../fixtures/integration_verbose.sh")
	IntegrationPrintsErr     = testhelp.Script("../fixtures/integration_err.sh")
	BasicCmd                 = testhelp.Script("../fixtures/basic_cmd.sh")
	ErrorCmd                 = testhelp.Script("../fixtures/error_cmd.sh")
	BlockedCmd               = testhelp.Script("../fixtures/blocked_cmd.sh")
	FileContentsCmd          = testhelp.Script("../fixtures/filecontents.sh")
	FileContentsWithArgCmd   = testhelp.Script("../fixtures/filecontents_witharg.sh")
	FileContentsFromEnvCmd   = testhelp.Script("../fixtures/filecontents_fromenv.sh")
	EchoFromEnv              = testhelp.Script("../fixtures/echo_from_env.sh")
)

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
