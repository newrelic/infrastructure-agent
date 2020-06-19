// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixtures

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
)

var (
	IntegrationScript        = testhelp.Script("..\\fixtures\\integration.ps1")
	IntegrationVerboseScript = testhelp.Script("..\\fixtures\\integration_verbose.ps1")
	IntegrationPrintsErr     = testhelp.Script("..\\fixtures\\integration_err.ps1")
	BasicCmd                 = testhelp.Script("..\\fixtures\\basic_cmd.ps1")
	ErrorCmd                 = testhelp.Script("..\\fixtures\\error_cmd.ps1")
	BlockedCmd               = testhelp.Script("..\\fixtures\\blocked_cmd.ps1")
	FileContentsWithArgCmd   = testhelp.Script("..\\fixtures\\filecontents_witharg.ps1")
	// at the moment, unsupported, as they use env vars with Powershell. Left here to avoid compile errors
	FileContentsCmd        = testhelp.Script("unsupported-test-case")
	FileContentsFromEnvCmd = testhelp.Script("unsupported-test-case")
)
