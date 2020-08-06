// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sender

import (
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"testing"
)

func TestNewContainerisedClient(t *testing.T) {

	type args struct {
		apiVersion  string
		containerID string
	}
	tests := []struct {
		name    string
		args    args
		wantC   bool
		wantErr bool
	}{
		{"no container id fails", args{apiVersion: "", containerID: ""}, false, true},
		{"no api version does not fail", args{apiVersion: "", containerID: "123"}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !helpers.IsDockerRunning() {
				t.Skip("docker required for this test suite")
			}
			gotC, err := NewContainerisedClient(tt.args.apiVersion, tt.args.containerID)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewContainerisedClient() error = %v, want %v", err, tt.wantErr)
				return
			}
			if (gotC != nil) != tt.wantC {
				t.Errorf("NewContainerisedClient() client = %v, want %v", gotC, tt.wantC)
			}
		})
	}
}
