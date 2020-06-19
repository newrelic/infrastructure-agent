// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package fixtures

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
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
)
