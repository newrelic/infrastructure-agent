// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixtures

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
)

var (
	IntegrationScript        = testhelp.Script("..\\fixtures\\integration.sh")
	IntegrationVerboseScript = testhelp.Script("..\\fixtures\\integration_verbose.sh")
	IntegrationPrintsErr     = testhelp.Script("..\\fixtures\\integration_err.sh")
	BasicCmd                 = testhelp.Script("..\\fixtures\\basic_cmd.sh")
	ErrorCmd                 = testhelp.Script("..\\fixtures\\error_cmd.sh")
	BlockedCmd               = testhelp.Script("..\\fixtures\\blocked_cmd.sh")
	FileContentsWithArgCmd   = testhelp.Script("..\\fixtures\\filecontents_witharg.sh")
	SleepCmd                 = testhelp.Script("..\\fixtures\\sleep.sh")
	// at the moment, unsupported, as they use env vars with Powershell. Left here to avoid compile errors
	FileContentsCmd        = testhelp.Script("unsupported-test-case")
	FileContentsFromEnvCmd = testhelp.Script("unsupported-test-case")
	EchoFromEnv            = testhelp.Script("unsupported-test-case")
)
