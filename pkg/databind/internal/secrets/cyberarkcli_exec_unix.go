// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build unix

package secrets

import "os/exec"

func (g *cyberArkCLIGatherer) cyberArkExecCommand() *exec.Cmd {
	return cyberArkExecCommand(g.cfg.CLI, "GetPassword", "-p", "AppDescs.AppID="+g.cfg.AppID, "-p", "Query=Safe="+g.cfg.Safe+";Folder="+g.cfg.Folder+";Object="+g.cfg.Object, "-o", "Password")
}
