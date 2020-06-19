// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"net/http"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_newTransport_RoundTrip(t *testing.T) {
	licenseKey := "myLicenseKey"
	entityID := entity.ID(13)

	req := &http.Request{Header: make(http.Header)}
	req.Header.Add("Api-Key", licenseKey)

	rt := new(roundTripperSpy)
	rt.On("RoundTrip", req).Return().Run(func(args mock.Arguments) {
		req := args.Get(0).(*http.Request)
		assert.Equal(t, licenseKey, req.Header.Get("X-License-Key"))
		assert.Empty(t, req.Header.Get("Api-Key"), "api key header should be removed")
		assert.Equal(t, entityID.String(), req.Header.Get("Infra-Agent-Entity-Id"))
	})

	fakeIdProvide := func() entity.Identity {
		return entity.Identity{
			ID: entityID,
		}
	}

	_, _ = newTransport(rt, licenseKey, fakeIdProvide).RoundTrip(req)
	rt.AssertExpectations(t)
}

type roundTripperSpy struct {
	mock.Mock
}

func (m *roundTripperSpy) RoundTrip(req *http.Request) (*http.Response, error) {
	m.Called(req)
	return &http.Response{}, nil
}
