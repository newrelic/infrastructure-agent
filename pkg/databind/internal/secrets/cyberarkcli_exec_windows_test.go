// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows

package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_cyberArkExecCommand(t *testing.T) {
	t.Parallel()
	cliStruct := CyberArkCLI{
		CLI:    `C:\Program Files (x86)\CyberArk\ApplicationPasswordSdk\CLIPasswordSDK.exe`,
		AppID:  "appid",
		Safe:   "safe",
		Folder: "folder",
		Object: "object",
	}

	gatherer := cyberArkCLIGatherer{cfg: &cliStruct}
	cmd := gatherer.cyberArkExecCommand()
	assert.Equal(t, `C:\Program Files (x86)\CyberArk\ApplicationPasswordSdk\CLIPasswordSDK.exe GetPassword /p AppDescs.AppID=appid /p Query=Safe=safe;Folder=folder;Object=object /o Password`, cmd.String())
}
