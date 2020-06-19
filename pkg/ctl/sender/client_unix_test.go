// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package sender

import (
	"context"

	"github.com/newrelic/infrastructure-agent/pkg/ipc"

	"github.com/newrelic/infrastructure-agent/internal/os/api/signals"
	"github.com/stretchr/testify/assert"

	"os"
	"os/signal"
	"sync"
	"testing"
	"time"
)

func TestNewProcClient(t *testing.T) {
	tests := []struct {
		name       string
		agentPID   int
		wantClient bool
		wantErr    bool
	}{
		{"no pid", 0, false, true},
		{"non exisiting pid", 99999, false, true},
		{"exisiting pid", os.Getpid(), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotC, err := NewClient(tt.agentPID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProcClient() error = %v, want %v", err, tt.wantErr)
				return
			}
			if (gotC != nil) != tt.wantClient {
				t.Errorf("NewProcClient() client = %v, want %v", gotC, tt.wantClient)
			}
		})
	}
}

func Test_procClient_EnableVerbose(t *testing.T) {
	// signal listener
	var receivedSignal os.Signal
	wg := sync.WaitGroup{}
	wg.Add(1)

	sC := make(chan os.Signal, 1)
	signal.Notify(sC, signals.Notification)
	go func() {
		select {
		case receivedSignal = <-sC:
		case <-time.After(1000 * time.Millisecond): // signaling on busy nodes takes time
		}
		wg.Done()
	}()

	c, err := NewClient(os.Getpid())
	assert.NoError(t, err)
	assert.NoError(t, c.Notify(context.Background(), ipc.EnableVerboseLogging))
	wg.Wait()
	// verbose signal was sent
	assert.Equal(t, signals.Notification, receivedSignal)
}
