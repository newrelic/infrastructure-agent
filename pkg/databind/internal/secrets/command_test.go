// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"reflect"
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
			name: "Test simple 'echo' command",
			command: &Command{
				CmdPath: "echo",
				CmdArgs: []string{"test"},
			},
			want:    "test",
			wantErr: false,
		},
		{
			name: "Test command with error",
			command: &Command{
				CmdPath: "unknown_command",
				CmdArgs: []string{"unneeded_arg"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Command with no args",
			command: &Command{
				CmdPath: "echo",
				CmdArgs: []string{},
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "Test valid JSON output",
			command: &Command{
				CmdPath: "echo",
				CmdArgs: []string{"{\"testKey\": \"testVal\"}"},
			},
			want:    map[string]any{"testKey": "testVal"},
			wantErr: false,
		},
		{
			name: "Test nested JSON",
			command: &Command{
				CmdPath: "echo",
				CmdArgs: []string{"{\"testKey\": {\"testKey2\": \"testVal\"}}"},
			},
			want:    map[string]any{"testKey": map[string]any{"testKey2": "testVal"}},
			wantErr: false,
		},
		{
			name: "Test JSON with lists",
			command: &Command{
				CmdPath: "echo",
				CmdArgs: []string{"{\"testKey\": [\"testVal1\", \"testVal2\"]}"},
			},
			want:    map[string]any{"testKey": []any{"testVal1", "testVal2"}},
			wantErr: false,
		},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, []commandTestCase{
			{
				name: "Test 'echo' command with arguments",
				command: &Command{
					CmdPath: "echo",
					CmdArgs: []string{"-n", "test2"},
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
			if !reflect.DeepEqual(res, testCase.want) {
				t.Errorf("CommandGatherer() = %v, want %v", res, testCase.want)
			}
		})
	}
}
