// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixtures

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
)

var (
	SimpleGoFile        = testhelp.WrapScriptPath("fixtures", "simple", "simple.go")
	EnvironmentGoFile   = testhelp.WrapScriptPath("fixtures", "environment", "environment_verbose.go")
	ProtocolV4GoFile    = testhelp.WrapScriptPath("fixtures", "protocol_v4", "protocol_v4.go")
	ValidYAMLGoFile     = testhelp.WrapScriptPath("fixtures", "validyaml", "validyaml.go")
	LongTimeGoFile      = testhelp.WrapScriptPath("fixtures", "longtime", "longtime.go")
	LongRunningHBGoFile = testhelp.WrapScriptPath("fixtures", "longrunning_hb", "longrunning_hb.go")
	HugeGoFile          = testhelp.WrapScriptPath("fixtures", "huge", "huge.go")
)
