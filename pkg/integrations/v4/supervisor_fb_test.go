// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"path"
	"testing"

	executor2 "github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFBSupervisorConfig_IsLogForwarderAvailable(t *testing.T) {
	// GIVEN
	file, err := ioutil.TempFile("", "nr_fb_config")
	if err != nil {
		assert.FailNow(t, "Could not create temporary testing file")
	}
	existing := file.Name()
	nonExisting := "non-existing-file"

	// GIVEN / THEN
	tests := []struct {
		name string
		cfg  FBSupervisorConfig
		want bool
	}{
		{
			"incorrect: all non-existing",
			FBSupervisorConfig{
				FluentBitExePath:     nonExisting,
				FluentBitNRLibPath:   nonExisting,
				FluentBitParsersPath: nonExisting,
				ConfTemporaryFolder:  os.TempDir(),
			},
			false,
		},
		{
			"incorrect: NR lib and parsers do not exist",
			FBSupervisorConfig{
				FluentBitExePath:     existing,
				FluentBitNRLibPath:   nonExisting,
				FluentBitParsersPath: nonExisting,
				ConfTemporaryFolder:  os.TempDir(),
			},
			false,
		},
		{
			"incorrect: parsers doesn't exist",
			FBSupervisorConfig{
				FluentBitExePath:     existing,
				FluentBitNRLibPath:   existing,
				FluentBitParsersPath: nonExisting,
				ConfTemporaryFolder:  os.TempDir(),
			},
			false,
		},
		{
			"correct configuration",
			FBSupervisorConfig{
				FluentBitExePath:     existing,
				FluentBitNRLibPath:   existing,
				FluentBitParsersPath: existing,
				ConfTemporaryFolder:  os.TempDir(),
			},
			true,
		},
	}

	// WHEN
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			available := tt.cfg.IsLogForwarderAvailable()

			assert.Equal(t, tt.want, available)
		})
	}

	// Teardown
	file.Close()
	if err := os.Remove(existing); err != nil {
		assert.FailNow(t, "Could not remove temporary test file")
	}
}

func TestFBSupervisorConfig_LicenseKeyShouldBePassedAsEnvVar(t *testing.T) {
	t.Parallel()

	fbConf := FBSupervisorConfig{ConfTemporaryFolder: os.TempDir()}
	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}
	hostnameResolver := testhelpers.NewFakeHostnameResolver("full_hostname", "short_hostname", nil)
	license := "some_license"
	c := config.LogForward{License: license, Troubleshoot: config.Troubleshoot{Enabled: true}}

	confLoader := logs.NewFolderLoader(c, agentIdentity, hostnameResolver)
	executorBuilder := buildFbExecutor(fbConf, confLoader)

	exec, err := executorBuilder()
	require.NoError(t, err)

	assert.Contains(t, exec.(*executor2.Executor).Cfg.Environment, "NR_LICENSE_KEY_ENV_VAR")       // nolint:forcetypeassert
	assert.Equal(t, exec.(*executor2.Executor).Cfg.Environment["NR_LICENSE_KEY_ENV_VAR"], license) //nolint:forcetypeassert
}

func Test_ConfigTemporaryFolderCreation(t *testing.T) {
	t.Parallel()

	randNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	assert.NoError(t, err)
	termporaryFolderPath := path.Join(os.TempDir(), fmt.Sprintf("ConfigTemporaryFolderCreation_%d", randNumber))
	fmt.Println(termporaryFolderPath)
	defer func() {
		os.Remove(termporaryFolderPath)
	}()

	fbConf := FBSupervisorConfig{ConfTemporaryFolder: termporaryFolderPath}
	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}
	hostnameResolver := testhelpers.NewFakeHostnameResolver("full_hostname", "short_hostname", nil)
	c := config.LogForward{Troubleshoot: config.Troubleshoot{Enabled: true}}

	confLoader := logs.NewFolderLoader(c, agentIdentity, hostnameResolver)
	executorBuilder := buildFbExecutor(fbConf, confLoader)

	_, err = executorBuilder()
	require.NoError(t, err)
	assert.DirExists(t, termporaryFolderPath)
}

// nolint:paralleltest
func TestRemoveFbConfigTempFiles(t *testing.T) {
	configFiles := []struct {
		name    string
		content string
	}{
		{"nr_fb_config1", "nr_fb_lua_filter1,nr_fb_lua_filter2"},
		{"nr_fb_config2", ""},
		{"nr_fb_config3", "nr_fb_lua_filter3"},
		{"nr_fb_config4", "nr_fb_lua_filter0"},
		{"nr_fb_config5", ""},
		{"nr_fb_config6", ""},
		{"nr_fb_lua_filter1", ""},
		{"nr_fb_lua_filter2", ""},
		{"nr_fb_lua_filter3", ""},
		{"nr_fb_lua_filter4", ""},
	}

	tests := []struct {
		name                     string
		maxNumConfFiles          int
		expectedRemovedConfFiles []string
		expectedKeptConfFiles    []string
		wantErr                  bool
	}{
		{
			name:                     "No config files are removed",
			maxNumConfFiles:          10,
			expectedRemovedConfFiles: []string{},
			expectedKeptConfFiles:    []string{"nr_fb_config1", "nr_fb_config2", "nr_fb_config3", "nr_fb_config4", "nr_fb_config5", "nr_fb_config6", "nr_fb_lua_filter1", "nr_fb_lua_filter2", "nr_fb_lua_filter3", "nr_fb_lua_filter4"},
			wantErr:                  false,
		},
		{
			name:                     "Config file 1 and config lua files 1 and 2 are removed",
			maxNumConfFiles:          5,
			expectedRemovedConfFiles: []string{"nr_fb_config1", "nr_fb_lua_filter1", "nr_fb_lua_filter2"},
			expectedKeptConfFiles:    []string{"nr_fb_config2", "nr_fb_config3", "nr_fb_config4", "nr_fb_config5", "nr_fb_config6", "nr_fb_lua_filter3", "nr_fb_lua_filter4"},
			wantErr:                  false,
		},
		{
			name:                     "Config files 1, 2 and 3 and config lua files 1, 2 and 3 are removed",
			maxNumConfFiles:          3,
			expectedRemovedConfFiles: []string{"nr_fb_config1", "nr_fb_config2", "nr_fb_config3", "nr_fb_lua_filter1", "nr_fb_lua_filter2", "nr_fb_lua_filter3"},
			expectedKeptConfFiles:    []string{"nr_fb_config4", "nr_fb_config5", "nr_fb_config6", "nr_fb_lua_filter4"},
			wantErr:                  false,
		},
		{
			name:                     "Config files 1, 2, 3, 4 and 5 and config lua files 1, 2 and 3 are removed. Error removing non-existing lua file 0 referenced by config file 4.",
			maxNumConfFiles:          1,
			expectedRemovedConfFiles: []string{"nr_fb_config1", "nr_fb_config2", "nr_fb_config3", "nr_fb_config4", "nr_fb_config5", "nr_fb_lua_filter1", "nr_fb_lua_filter2", "nr_fb_lua_filter3"},
			expectedKeptConfFiles:    []string{"nr_fb_config6", "nr_fb_lua_filter4"},
			wantErr:                  true,
		},
	}

	for _, test := range tests {
		// create temp directory and set it as default directory to use for temporary files
		tmpDir := t.TempDir()
		t.Setenv("TMPDIR", tmpDir)

		t.Run(test.name, func(t *testing.T) {
			// create config files in temp directory
			for _, file := range configFiles {
				addFile(t, tmpDir, file.name, file.content)
			}

			got, err := removeFbConfigTempFiles(test.maxNumConfFiles)
			if (err != nil) != test.wantErr {
				t.Errorf("removeFbConfigTempFiles() error = %v, wantErr %v", err, test.wantErr)

				return
			}

			// read the remaining config file names from the temp directory
			files, err := os.Open(tmpDir)
			require.NoError(t, err)
			keptConfTempFilenames, err := files.Readdirnames(0)
			require.NoError(t, err)

			assert.ElementsMatchf(t, test.expectedRemovedConfFiles, got, "Config files removed do not match")
			assert.ElementsMatchf(t, test.expectedKeptConfFiles, keptConfTempFilenames, "Config files kept do not match")
		})
	}
}

func addFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	filePath := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(filePath, []byte(contents), 0o0600))
}
