// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sender

import (
	"runtime"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

func TestContainerdClient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping flaky test on windows")
	}

	type args struct {
		containerID string
		namespace   string
	}
	tests := []struct {
		name    string
		args    args
		wantC   bool
		wantErr bool
	}{
		{"no container id fails", args{containerID: "", namespace: ""}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !helpers.IsDockerRunning() {
				t.Skip("docker required for this test suite")
			}
			gotC, err := NewContainerdClient(tt.args.containerID, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDockerClient() error = %v, want %v", err, tt.wantErr)
				return
			}
			if (gotC != nil) != tt.wantC {
				t.Errorf("NewDockerClient() client = %v, want %v", gotC, tt.wantC)
			}
		})
	}
}
