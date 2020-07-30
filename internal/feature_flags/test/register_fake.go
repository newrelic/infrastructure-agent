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
