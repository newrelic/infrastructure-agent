// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixtures

import (
	"path"
	"runtime"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
)

var (
	BasicCmdWithSpace  = testhelp.Script(path.Join("..", "fixtures", "basic cmd"+getExtension()))
	TimestampDiscovery = testhelp.WrapScriptPath("..", "fixtures", "discoverer", "discoverer.go")
	// The following test can't use `testhelp.WrapScriptPath` as it has arguments passed to it
	InventoryGoFile     = testhelp.Script(path.Join("..", "fixtures", "inventory", "inventory.go"))
	InventoryScriptFile = testhelp.Script(path.Join("..", "fixtures", "inventory", "inventory.sh"))
)

func getExtension() string {
	if runtime.GOOS == "windows" {
		return ".ps1"
	}
	return ".sh"
}
