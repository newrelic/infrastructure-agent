//go:build !windows
// +build !windows

/*
 * Copyright 2021 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package logs

func addOSDependantConfig(fbOSConf FBOSConfig) FBOSConfig {
	fbOSConf.ForceUseANSI = true
	return fbOSConf
}
