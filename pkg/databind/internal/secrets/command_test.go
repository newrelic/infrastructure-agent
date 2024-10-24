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

func TestCommandGatherer(t *testing.T) {
	// Arrange
	tests := []commandTestCase{
		{
			name: "Simple 'echo' command",
			command: &Command{
				Path:           "echo",
				Args:           []string{"test"},
				PassthroughEnv: nil,
			},
			want:    "test",
			wantErr: false,
		},
		{
			name: "Command with error",
			command: &Command{
				Path:           "unknown_command",
				Args:           []string{"unneeded_arg"},
				PassthroughEnv: nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Command with no args",
			command: &Command{
				Path:           "echo",
				Args:           nil,
				PassthroughEnv: nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid JSON output",
			command: &Command{
				Path:           "echo",
				Args:           []string{"{\"testKey\": \"testVal\"}"},
				PassthroughEnv: nil,
			},
			want:    map[string]any{"testKey": "testVal"},
			wantErr: false,
		},
		{
			name: "Nested JSON",
			command: &Command{
				Path:           "echo",
				Args:           []string{"{\"testKey\": {\"testKey2\": \"testVal\"}}"},
				PassthroughEnv: nil,
			},
			want:    map[string]any{"testKey": map[string]any{"testKey2": "testVal"}},
			wantErr: false,
		},
		{
			name: "JSON with lists",
			command: &Command{
				Path:           "echo",
				Args:           []string{"{\"testKey\": [\"testVal1\", \"testVal2\"]}"},
				PassthroughEnv: nil,
			},
			want:    map[string]any{"testKey": []any{"testVal1", "testVal2"}},
			wantErr: false,
		},
		{
			name: "cmdResponse",
			command: &Command{
				Path:           "echo",
				Args:           []string{"{\"ttl\": \"1h\", \"data\": {\"testKey\": \"testVal\"}}"},
				PassthroughEnv: nil,
			},
			want: &cmdResponse{
				CmdData: map[string]any{"testKey": "testVal"},
				CmdTTL:  "1h",
			},
			wantErr: false,
		},
		{
			name: "cmdResponseWithDataWithoutTTL",
			command: &Command{
				Path:           "echo",
				Args:           []string{"{\"data\": {\"testKey\": \"testVal\"}}"},
				PassthroughEnv: nil,
			},
			want: &cmdResponse{
				CmdData: map[string]any{"testKey": "testVal"},
				CmdTTL:  "",
			},
			wantErr: false,
		},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, []commandTestCase{
			{
				name: "'echo' command with arguments",
				command: &Command{
					Path:           "echo",
					Args:           []string{"-n", "test2"},
					PassthroughEnv: nil,
				},
				want:    "test2",
				wantErr: false,
			},
			{
				name: "Command with arguments and no passthru environment variables",
				command: &Command{
					Path:           "env",
					Args:           []string{},
					PassthroughEnv: nil,
				},
				want:    nil,
				wantErr: true,
			},
			{
				name: "Command with arguments and passthru environment variables",
				command: &Command{
					Path:           "env",
					Args:           []string{},
					PassthroughEnv: []string{"TEST_ENV_VAR"},
				},
				want:    "TEST_ENV_VAR=test",
				wantErr: false,
			},
			{
				name: "cmdResponse with env",
				command: &Command{
					Path: "sh",
					// Careful with the escaping sequences here!
					Args:           []string{"-c", `echo \{\"ttl\":\"1h\",\"data\":\{\"testKey\":\"$TEST_ENV_VAR\"\}\}`},
					PassthroughEnv: []string{"TEST_ENV_VAR"},
				},
				want: &cmdResponse{
					CmdData: map[string]any{"testKey": "test"},
					CmdTTL:  "1h",
				},
				wantErr: false,
			},
		}...)
	}

	for _, tt := range tests { //nolint:paralleltest
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			for _, env := range testCase.command.PassthroughEnv {
				t.Setenv(env, "test")
			}
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

func Test_parsePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
		want    any
		wantErr bool
	}{
		{
			name:    "Simple 'echo' command",
			payload: []byte("test"),
			want:    "test",
			wantErr: false,
		},
		{
			name:    "Valid JSON output",
			payload: []byte("{\"testKey\": \"testVal\"}"),
			want:    map[string]any{"testKey": "testVal"},
			wantErr: false,
		},
		{
			name:    "Valid cmdResponse output with data field set to a string",
			payload: []byte("{\"data\": \"testVal\"}"),
			want: &cmdResponse{
				CmdData: map[string]any{"testVal": "testVal"},
				CmdTTL:  "",
			},
			wantErr: false,
		},
		{
			name:    "Valid cmdResponse output with data field set to a map[string]any",
			payload: []byte("{\"data\": {\"testKey\": \"testVal\"}}"),
			want: &cmdResponse{
				CmdData: map[string]any{"testKey": "testVal"},
				CmdTTL:  "",
			},
			wantErr: false,
		},
		{
			name:    "Valid cmdResponse output with data field set to a map[string]any and ttl field",
			payload: []byte("{\"data\": {\"testKey\": \"testVal\"}, \"ttl\": \"30s\"}"),
			want: &cmdResponse{
				CmdData: map[string]any{"testKey": "testVal"},
				CmdTTL:  "30s",
			},
			wantErr: false,
		},
		{
			name:    "Invalid cmdResponse (no data field) but valid JSON output",
			payload: []byte("{\"ttl\": \"30s\", \"randomField\": {\"testKey\": \"testVal\"}}"),
			want: map[string]any{
				"ttl": "30s",
				"randomField": map[string]any{
					"testKey": "testVal",
				},
			},
			wantErr: false,
		},
		{
			name:    "Invalid JSON output",
			payload: []byte("{\""),
			want:    "{\"",
			wantErr: false,
		},
		{
			name:    "Invalid cmdResponse output (invalid data field)",
			payload: []byte("{\"data\": [\"testVal\"], \"ttl\": \"30s\"}"),
			want: map[string]any{
				"data": []any{"testVal"},
				"ttl":  "30s",
			},
			wantErr: false,
		},
		{
			name:    "Invalid input",
			payload: []byte(""), // empty payload
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := parsePayload(testCase.payload)

			// Assert
			if (err != nil) != testCase.wantErr {
				t.Fatalf("CommandGatherer() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("CommandGatherer() = %v, want %v", got, testCase.want)
			}
		})
	}
}

//nolint:paralleltest
func Test_setCmdEnv(t *testing.T) {
	type args struct {
		env []string
	}
	tests := []struct {
		name string
		args args
		env  map[string]string
		want []string
	}{
		{
			name: "Simple env var",
			args: args{
				env: []string{"TEST_ENV"},
			},
			env:  map[string]string{"TEST_ENV": "test"},
			want: []string{"TEST_ENV=test"},
		},
		{
			name: "Multiple env vars",
			args: args{
				env: []string{"TEST_ENV", "TEST_ENV2"},
			},
			env:  map[string]string{"TEST_ENV": "test", "TEST_ENV2": "test"},
			want: []string{"TEST_ENV=test", "TEST_ENV2=test"},
		},
		{
			name: "Env var not found",
			args: args{
				env: []string{"TEST_ENV", "TEST_ENV2"},
			},
			env:  map[string]string{"TEST_ENV": "test"},
			want: []string{"TEST_ENV=test"},
		},
		{
			name: "No matches",
			args: args{
				env: []string{"TEST_ENV1"},
			},
			env:  map[string]string{"TEST_ENV2": "test", "TEST_ENV3": "test"},
			want: []string{},
		},
		{
			name: "Empty env var",
			args: args{
				env: []string{"TEST_ENV", "TEST_ENV2"},
			},
			env:  map[string]string{"TEST_ENV": "test", "TEST_ENV2": ""},
			want: []string{"TEST_ENV=test", "TEST_ENV2="},
		},
		{
			name: "Env var regexp",
			args: args{
				env: []string{"TEST_ENV*"},
			},
			env:  map[string]string{"TEST_ENV": "test", "TEST_ENV2": "test", "TEST_ENV3": "test"},
			want: []string{"TEST_ENV=test", "TEST_ENV2=test", "TEST_ENV3=test"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			for k, v := range testCase.env {
				t.Setenv(k, v)
			}
			if got := setCmdEnv(testCase.args.env); !slicesHaveSameContent(t, got, testCase.want) {
				t.Errorf("setCmdEnv() = %v, want %v", got, testCase.want)
			}
		})
	}
}

// slicesHaveSameContent checks if two slices have the same content disregarding order.
func slicesHaveSameContent(t *testing.T, got, want []string) bool {
	t.Helper()

	if len(got) != len(want) {
		return false
	}
	count1 := make(map[string]bool)
	count2 := make(map[string]bool)

	for _, v := range got {
		count1[v] = true
	}

	for _, v := range want {
		count2[v] = true
	}

	for k, v := range count1 {
		if count2[k] != v {
			return false
		}
	}

	return true
}

//nolint:paralleltest
func Test_runCommand(t *testing.T) {
	type args struct {
		cmd *Command
	}
	type testCase struct {
		name    string
		args    args
		env     map[string]string
		want    []byte
		wantErr bool
	}
	tests := []testCase{
		{
			name: "Command with arguments",
			args: args{
				cmd: &Command{
					Path:           "echo",
					Args:           []string{"test"},
					PassthroughEnv: nil,
				},
			},
			env:     nil,
			want:    []byte("test"),
			wantErr: false,
		},
		{
			name: "Pass env but command does not need env",
			args: args{
				cmd: &Command{
					Path:           "echo",
					Args:           []string{"test"},
					PassthroughEnv: []string{"TEST_ENV"},
				},
			},
			env:     map[string]string{"TEST_ENV": "testFromEnv"},
			want:    []byte("test"),
			wantErr: false,
		},
		{
			name: "Empty responses are invalid",
			args: args{
				cmd: &Command{
					Path:           "echo",
					Args:           nil,
					PassthroughEnv: nil,
				},
			},
			env:     nil,
			want:    nil,
			wantErr: true,
		},
	}

	if runtime.GOOS != "windows" {
		tests = append(tests, []testCase{
			{
				name: "Command with arguments and env (Unix))",
				args: args{
					cmd: &Command{
						Path:           "sh",
						Args:           []string{"-c", "echo $TEST_ENV"},
						PassthroughEnv: []string{"TEST_ENV"},
					},
				},
				env:     map[string]string{"TEST_ENV": "testFromEnv"},
				want:    []byte("testFromEnv"),
				wantErr: false,
			},
			{
				name: "Command with JSON arguments and env (Unix))",
				args: args{
					cmd: &Command{
						Path:           "sh",
						Args:           []string{"-c", "echo {\\\"data\\\": \\\"$TEST_ENV\\\"}"},
						PassthroughEnv: []string{"TEST_ENV"},
					},
				},
				env:     map[string]string{"TEST_ENV": "testFromEnv"},
				want:    []byte("{\"data\": \"testFromEnv\"}"),
				wantErr: false,
			},
		}...)
	} else {
		tests = append(tests, []testCase{
			{
				name: "Command with arguments and env (Windows)",
				args: args{
					cmd: &Command{
						Path:           "cmd",
						Args:           []string{"/c", "echo %TEST_ENV%"},
						PassthroughEnv: []string{"TEST_ENV"},
					},
				},
				env:     map[string]string{"TEST_ENV": "testFromEnv"},
				want:    []byte("testFromEnv"),
				wantErr: false,
			},
		}...)
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			for k, v := range testCase.env {
				t.Setenv(k, v)
			}
			got, err := runCommand(testCase.args.cmd)
			if (err != nil) != testCase.wantErr {
				t.Errorf("runCommand() error = %v, wantErr %v", err, testCase.wantErr)

				return
			}
			if !reflect.DeepEqual(got, testCase.want) {
				// Represent as string for readability
				t.Errorf("runCommand() = %s, want %s", got, testCase.want)
			}
		})
	}
}
