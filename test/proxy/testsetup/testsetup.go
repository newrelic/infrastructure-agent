// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testsetup

import (
	"fmt"
)

// Some data that may be also bundled in the testing configuration files (Dockerfiles, bash scripts...)
const (
	CollectorPort        = 4444
	AgentPort            = 4445
	HttpProxyPort        = 3128
	ActualHttpsProxyPort = 3129
	HttpProxyName        = "http-proxy"
	ActualHttpsProxyName = "https-proxy"
	CaBundleDir          = "/cabundle"
	CollectorCertFile    = CaBundleDir + "/collector.pem"
	CollectorKeyFile     = CaBundleDir + "/collector.key"
)

// Addresses to be run inside a container
var (
	HttpProxy        = fmt.Sprintf("http://%s:%d", HttpProxyName, HttpProxyPort)
	HttpsProxy       = fmt.Sprintf("https://%s:%d", HttpProxyName, HttpProxyPort)
	ActualHttpsProxy = fmt.Sprintf("https://%s:%d", ActualHttpsProxyName, ActualHttpsProxyPort)
)

// Addresses to be run from the tests (through the exposed ports)
var (
	AgentRestart       = fmt.Sprintf("http://localhost:%d/restart", AgentPort)
	Collector          = fmt.Sprintf("https://localhost:%d", CollectorPort)
	CollectorCleanup   = Collector + "/cleanup"
	CollectorNextEvent = Collector + "/nextevent"
	CollectorUsedProxy = Collector + "/lastproxy"
)
