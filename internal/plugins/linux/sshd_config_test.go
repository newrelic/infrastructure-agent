// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	check "gopkg.in/check.v1"
)

type SshdConfigSuite struct{}

var _ = check.Suite(&SshdConfigSuite{})

func (suite *SshdConfigSuite) TestParseSshdConfig(checks *check.C) {
	testInputs := map[string]struct {
		configText     string
		expectedConfig map[string]string
	}{
		"all_on": {
			configText: `
PermitRootLogin without-password
# a comment
SomeCoolSettings     no
PermitEmptyPasswords yes # a comment afterwards
 PasswordAuthentication      yes
Derp no
ChallengeResponseAuthentication yes
			`,
			expectedConfig: map[string]string{
				"PermitRootLogin":                 "without-password",
				"PermitEmptyPasswords":            "yes",
				"PasswordAuthentication":          "yes",
				"ChallengeResponseAuthentication": "yes",
			},
		},
		"tricky_comment": {
			configText: `
PermitRootLogin no
# PermitRootLogin yes
`,
			expectedConfig: map[string]string{
				"PermitRootLogin": "no",
			},
		},
	}

	for _, inputs := range testInputs {
		config, err := parseSshdConfig(inputs.configText)
		checks.Check(err, check.IsNil)
		checks.Check(config, check.DeepEquals, inputs.expectedConfig)
	}
}

func TestResolveSshdConfigPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		defaultExists  bool
		fallbackExists bool
		wantfallback   bool
	}{
		{
			name:           "only the default file exists",
			defaultExists:  true,
			fallbackExists: false,
			wantfallback:   false,
		},
		{
			name:           "both exist: the default file wins",
			defaultExists:  true,
			fallbackExists: true,
			wantfallback:   false,
		},
		{
			name:           "only the fallback file exists: falls back to it",
			defaultExists:  false,
			fallbackExists: true,
			wantfallback:   true,
		},
		{
			name:           "neither exists: keeps the default path",
			defaultExists:  false,
			fallbackExists: false,
			wantfallback:   false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			defaultPath := filepath.Join(dir, "default_sshd_config")
			fallbackPath := filepath.Join(dir, "fallback_sshd_config")

			if testCase.defaultExists {
				require.NoError(t, os.WriteFile(defaultPath, []byte("PermitRootLogin no"), 0o600))
			}

			if testCase.fallbackExists {
				require.NoError(t, os.WriteFile(fallbackPath, []byte("PermitRootLogin yes"), 0o600))
			}

			want := defaultPath
			if testCase.wantfallback {
				want = fallbackPath
			}

			assert.Equal(t, want, resolveSshdConfigPath(defaultPath, fallbackPath))
		})
	}
}

// A stat failure that is not "does not exist" must not make the agent silently read the fallback file instead.
func TestResolveSshdConfigPath_KeepsdefaultPathOnOtherStatErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	notADirectory := filepath.Join(dir, "regular_file")
	require.NoError(t, os.WriteFile(notADirectory, []byte{}, 0o600))
	// stat fails with ENOTDIR, not ENOENT, because a path component is a regular file.
	defaultPath := filepath.Join(notADirectory, "sshd_config")
	fallbackPath := filepath.Join(dir, "fallback_sshd_config")
	require.NoError(t, os.WriteFile(fallbackPath, []byte("PermitRootLogin yes"), 0o600))

	assert.Equal(t, defaultPath, resolveSshdConfigPath(defaultPath, fallbackPath))
}
