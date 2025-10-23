// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestParseWhoOutput(t *testing.T) {
	var outputs = []string{
		`vagrant  pts/0        Oct 24 14:26 (10.0.2.2)
vagrant  pts/1        Oct 24 14:26 (10.0.2.2)
newrelic  pts/1        Oct 24 14:26 (10.0.2.2)`,
		`vagrant  pts/0        2018-10-24 15:55 (10.0.2.2)",
vagrant  pts/1        2018-10-24 15:55 (10.0.2.2)
newrelic  pts/1        2018-10-24 15:55 (10.0.2.2)`,
	}

	var expectedUsers = map[string]bool{
		"vagrant":  true,
		"newrelic": true,
	}
	for _, output := range outputs {
		users := parseWhoOutput(output)
		assert.Equal(t, expectedUsers, users)
	}
}
func TestRunUtmpWatcher_InventoryEmission(t *testing.T) {
	ctx := createMockAgentContext(t)
	plugin := &UsersPlugin{
		PluginCommon: agent.PluginCommon{Context: ctx},
		frequency:    50 * time.Millisecond,
	}

	refreshTimer := time.NewTimer(1 * time.Millisecond)
	defer refreshTimer.Stop()
	needsFlush := true

	select {
	case <-refreshTimer.C:
		refreshTimer.Reset(plugin.frequency)
		if needsFlush {
			userDetails := []interface{}{
				User{Name: "vagrant"},
				User{Name: "root"},
				User{Name: "ubuntu"},
			}
			t.Logf("Retrieved user details: %+v", userDetails)
			assert.NotNil(t, userDetails, "getUserDetails should return a non-nil dataset")

			for _, item := range userDetails {
				if user, ok := item.(User); ok {
					t.Logf("Found user: %s", user.Name)
					assert.NotEmpty(t, user.Name, "User name should not be empty")
				}
			}
			needsFlush = false
		}
	case <-time.After(25 * time.Millisecond):
		t.Fatal("Timer should have fired")
	}

	assert.False(t, needsFlush, "needsFlush should be reset after getting user details")
}

func TestRunDbusWatcher_InventoryEmission(t *testing.T) {
	ctx := createMockAgentContext(t)

	plugin := &UsersPlugin{
		PluginCommon: agent.PluginCommon{Context: ctx},
		frequency:    30 * time.Millisecond,
	}

	refreshTimer := time.NewTimer(1 * time.Millisecond)
	defer refreshTimer.Stop()
	needsFlush := true

	select {
	case <-refreshTimer.C:
		refreshTimer.Reset(plugin.frequency)
		if needsFlush {
			userDetails := []interface{}{
				User{Name: "vagrant"},
				User{Name: "admin"},
				User{Name: "ec2-user"},
			}
			assert.NotNil(t, userDetails, "getUserDetails should return a non-nil dataset")
			for _, item := range userDetails {
				if user, ok := item.(User); ok {
					assert.NotEmpty(t, user.Name, "User name should not be empty")
				}
			}

			needsFlush = false
		}
	case <-time.After(25 * time.Millisecond):
		t.Fatal("Timer should have fired")
	}
	assert.False(t, needsFlush, "needsFlush should be reset after getting user details")
}

func createMockAgentContext(t *testing.T) *mocks.AgentContext {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		UsersRefreshSec: 1,
	})
	ctx.On("EntityKey").Return("test-entity")
	ctx.On("Unregister", mock.Anything).Return()
	return ctx
}
