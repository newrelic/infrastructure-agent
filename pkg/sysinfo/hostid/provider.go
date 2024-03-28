// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package hostid

import "os"

const hostIDEnv = "NR_HOST_ID"

type Provider interface {
	Provide() (string, error)
}

type ProviderEnv struct{}

func NewProviderEnv() *ProviderEnv {
	return &ProviderEnv{}
}

func (p *ProviderEnv) Provide() (string, error) {
	// get host.it from env var
	hostID, _ := os.LookupEnv(hostIDEnv)

	return hostID, nil
}
