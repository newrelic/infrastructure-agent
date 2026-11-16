// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fromSecond = "from-second"

func TestReadFirstExisting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		files       map[string]string
		wantContent string
		wantErr     error
	}{
		{
			name:        "first path exists",
			files:       map[string]string{"first": "from-first", "second": fromSecond},
			wantContent: "from-first",
			wantErr:     nil,
		},
		{
			name:        "falls back to the second path",
			files:       map[string]string{"second": fromSecond},
			wantContent: fromSecond,
			wantErr:     nil,
		},
		{
			name:        "no path exists",
			files:       map[string]string{},
			wantContent: "",
			wantErr:     errNoSshdConfig,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range testCase.files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
			}

			content, err := readFirstExisting([]string{filepath.Join(dir, "first"), filepath.Join(dir, "second")})

			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantContent, string(content))
		})
	}
}

func TestReadFirstExisting_StopsOnOtherErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// a directory can not be read as a file: the error is not "not exist", so the next path must not be tried.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "first"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "second"), []byte(fromSecond), 0o600))

	_, err := readFirstExisting([]string{filepath.Join(dir, "first"), filepath.Join(dir, "second")})

	require.Error(t, err)
	assert.NotErrorIs(t, err, errNoSshdConfig)
}

func TestSshdConfigPaths_IncludesSles16Default(t *testing.T) {
	t.Setenv("HOST_ETC", "")

	assert.Equal(t, []string{"/etc/ssh/sshd_config", "/usr/etc/ssh/sshd_config"}, sshdConfigPaths())
}
