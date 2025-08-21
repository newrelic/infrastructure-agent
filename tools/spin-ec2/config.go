// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

type canaryConf struct {
	license            string
	agentVersion       string
	platform           string
	ansiblePassword    string
	prefix             string
	repo               string
	macstadiumUser     string
	macstadiumPass     string
	macstadiumSudoPass string
	ansibleForks       int
}

const defaultAnsibleForks = 5

func (c canaryConf) shouldProvisionLinux() bool {
	return c.platform == linux || c.platform == all
}

func (c canaryConf) shouldProvisionWindows() bool {
	return c.platform == windows || c.platform == all
}

func (c canaryConf) shouldProvisionMacos() bool {
	return c.platform == macos || c.platform == all
}
