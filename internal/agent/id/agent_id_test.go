// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package id

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	idC := NewContext(context.Background())
	identity := idC.agentIdentity.Load().(entity.Identity)
	assert.Empty(t, identity)
}

func TestContext_SetAgentID_WaitsUntilAnIDIsSet(t *testing.T) {
	c := NewContext(context.Background())

	var id entity.ID
	ready := make(chan entity.ID)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Wait()
		ready <- c.AgentID()
		close(ready)
	}()

	assert.Empty(t, id)

	c.SetAgentIdentity(newId(123))

	wg.Done()
	id = <-ready
	<-ready
	assert.Equal(t, entity.ID(123), id)
}

func TestContext_AgentID_CanBeSwitchedBackToEmpty(t *testing.T) {
	c := NewContext(context.Background())
	c.SetAgentIdentity(newId(1))
	c.SetAgentIdentity(newId(0))
	c.SetAgentIdentity(newId(2))
	_ = c.AgentID()
}

func TestContext_Notify(t *testing.T) {
	c := NewContext(context.Background())

	connectNotifications := make(chan struct{}, 1)
	c.Notify(connectNotifications, NotifyOnConnect)

	reconnectNotifications := make(chan struct{}, 1)
	c.Notify(reconnectNotifications, NotifyOnReconnect)

	allNotifications := make(chan struct{}, 1)
	c.Notify(allNotifications, NotifyOnConnect|NotifyOnReconnect)

	// Initial connect should trigger connect notification but not reconnect.
	c.SetAgentIdentity(newId(1))
	require.Equal(t, 1, len(connectNotifications), "Expected one connect notification")
	<-connectNotifications
	require.Equal(t, 0, len(reconnectNotifications), "Unexpected reconnect notification received")
	require.Equal(t, 1, len(allNotifications), "Expected one connect/reconnect notification")
	<-allNotifications

	// Reconnect.
	c.SetAgentIdentity(newId(2))
	require.Equal(t, 0, len(connectNotifications), "Unexpected connect notification")
	require.Equal(t, 1, len(reconnectNotifications), "Expected one reconnect notification")
	<-reconnectNotifications
	require.Equal(t, 1, len(allNotifications), "Expected one connect/reconnect notification")
	<-allNotifications

	// Same ID doesn't trigger notification.
	c.SetAgentIdentity(newId(2))
	require.Equal(t, 0, len(connectNotifications), "Unexpected connect notification")
	require.Equal(t, 0, len(reconnectNotifications), "Unexpected reconnect notification received")
	require.Equal(t, 0, len(allNotifications), "Unexpected connect/reconnect notification")
}

// test helper as only Identity constructor should be IdentityResponse.ToIdentity().
func newId(id int) entity.Identity {
	return entity.Identity{
		ID: entity.ID(id),
	}
}
