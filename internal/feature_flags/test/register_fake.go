// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package test

import (
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
)

type fakeRegisterClient struct {
	identityapi.RegisterClient
}

func NewFakeRegisterClient() identityapi.RegisterClient {
	return &fakeRegisterClient{}
}
