// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"runtime"
	"testing"
)

type commandTestCase struct {
	name    string
	command *Command
	want    any
	wantErr bool
}

func TestRunCommand(t *testing.T) {
	t.Parallel()
	// Arrange
	tests := []commandTestCase{
		{
			name: "Test 'echo' command",
			command: &Command{
				Key:     "test",
				CmdPath: "echo",
				CmdArgs: &[]string{"test"},
			},
			want:    "test",
			wantErr: false,
		},
		{
			name: "Test command with error",
			command: &Command{
				Key:     "test",
				CmdPath: "unknown_command",
				CmdArgs: &[]string{"unneeded_arg"},
			},
			want:    nil,
			wantErr: true,
		},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, []commandTestCase{
			{
				name: "Test 'echo' command with arguments",
				command: &Command{
					Key:     "test",
					CmdPath: "echo",
					CmdArgs: &[]string{"-n", "test2"},
				},
				want:    "test2",
				wantErr: false,
			},
		}...)
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			// Act
			g := CommandGatherer(testCase.command)
			res, err := g()

			// Assert
			if (err != nil) != testCase.wantErr {
				t.Fatalf("CommandGatherer() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if res != testCase.want {
				t.Errorf("CommandGatherer() = %v, want %v", res, testCase.want)
			}
		})
	}
}
