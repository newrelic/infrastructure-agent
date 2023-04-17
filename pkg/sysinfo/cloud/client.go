// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"net"
	"net/http"
	"time"
)

var (
	// limits the time spent establishing a TCP connection.
	defaultDialTimeout   = 2 * time.Second
	defaultDialKeepAlive = 30 * time.Second

	// decreased default's http client timeout to prevent the agent being initialized for 2 minutes.
	defaultClientTimeout = 30 * time.Second
)

// DRY function to construct a standard client for making cloud metadata calls that timeout quickly.
func clientWithFastTimeout(disableKeepAlive bool) *http.Client {
	return &http.Client{
		Timeout: defaultClientTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   defaultDialTimeout,
				KeepAlive: defaultDialKeepAlive,
			}).DialContext, // time out after 2 seconds => non-cloud instance.
			DisableKeepAlives: disableKeepAlive,
		},
	}
}
