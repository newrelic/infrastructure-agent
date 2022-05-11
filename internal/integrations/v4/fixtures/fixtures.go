// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixtures

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"path"
	"runtime"
)

var (
	BasicCmdWithSpace  = testhelp.Script(path.Join("..", "fixtures", "basic cmd"+getExtension()))
	TimestampDiscovery = testhelp.WrapScriptPath("..", "fixtures", "discoverer", "discoverer.go")
	// The following test can't use `testhelp.WrapScriptPath` as it has arguments passed to it
	InventoryGoFile = testhelp.Script(path.Join("..", "fixtures", "inventory", "inventory.go"))
)

func getExtension() string {
	if runtime.GOOS == "windows" {
		return ".ps1"
	}
	return ".sh"
}
