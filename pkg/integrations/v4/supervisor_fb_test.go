// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

//nolint:gci
import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
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
		cfg  fBSupervisorConfig
		want bool
	}{
		{
			"incorrect: all non-existing",
			fBSupervisorConfig{
				fluentBitExePath:     nonExisting,
				FluentBitNRLibPath:   nonExisting,
				FluentBitParsersPath: nonExisting,
				ConfTemporaryFolder:  os.TempDir(),
			},
			false,
		},
		{
			"incorrect: NR lib and parsers do not exist",
			fBSupervisorConfig{
				fluentBitExePath:     existing,
				FluentBitNRLibPath:   nonExisting,
				FluentBitParsersPath: nonExisting,
				ConfTemporaryFolder:  os.TempDir(),
			},
			false,
		},
		{
			"incorrect: parsers doesn't exist",
			fBSupervisorConfig{
				fluentBitExePath:     existing,
				FluentBitNRLibPath:   existing,
				FluentBitParsersPath: nonExisting,
				ConfTemporaryFolder:  os.TempDir(),
			},
			false,
		},
		{
			"correct configuration",
			fBSupervisorConfig{
				fluentBitExePath:     existing,
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

	fbConf := fBSupervisorConfig{ConfTemporaryFolder: os.TempDir()}
	bypassIsLogForwarderAvailable(t, &fbConf)

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

	assert.Contains(t, exec.(*executor.Executor).Cfg.Environment, "NR_LICENSE_KEY_ENV_VAR")       // nolint:forcetypeassert
	assert.Equal(t, exec.(*executor.Executor).Cfg.Environment["NR_LICENSE_KEY_ENV_VAR"], license) //nolint:forcetypeassert
}

func Test_ConfigTemporaryFolderCreation(t *testing.T) {
	t.Parallel()

	randNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	assert.NoError(t, err)

	termporaryFolderPath := path.Join(os.TempDir(), fmt.Sprintf("ConfigTemporaryFolderCreation_%d", randNumber))
	defer func() {
		os.Remove(termporaryFolderPath)
	}()

	fbConf := fBSupervisorConfig{ConfTemporaryFolder: termporaryFolderPath}
	bypassIsLogForwarderAvailable(t, &fbConf)

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

func TestRemoveFbConfigTempFiles(t *testing.T) {
	t.Parallel()
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

	for _, testItem := range tests {
		// Prevent the loop variable from being captured in the closure below
		test := testItem

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// create temp directory and set it as default directory to use for temporary files
			tmpDir, err := os.MkdirTemp("", "TestRemoveFbConfigTempFiles")
			defer os.RemoveAll(tmpDir)
			if err != nil {
				assert.FailNow(t, "Could not create temporary testing directory")
			}

			// create config files in temp directory
			for _, file := range configFiles {
				addFile(t, tmpDir, file.name, file.content)
			}

			got, err := removeFbConfigTempFiles(tmpDir, test.maxNumConfFiles)
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

//nolint:funlen
func TestNewSupervisorConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                 string
		ffEnabled            bool
		ffExists             bool
		agentDir             string
		integrationsDir      string
		loggingBinDir        string
		fluentBitExePath     string
		fluentBitNRLibPath   string
		fluentBitParsersPath string
		fbVerbose            bool
		expectedPathLinux    string
		expectedPathWindows  string
	}{
		{
			name:                "configuration should rule with no ff",
			fluentBitExePath:    "fluentBitExePath",
			expectedPathLinux:   "fluentBitExePath",
			expectedPathWindows: "fluentBitExePath",
		},
		{
			name:                "configuration should rule with no ff and loggin dir defined",
			loggingBinDir:       "loggingBinDir",
			fluentBitExePath:    "fluentBitExePath",
			expectedPathLinux:   "fluentBitExePath",
			expectedPathWindows: "fluentBitExePath",
		},
		{
			name:                "configuration should rule with ff disabled and loggin dir defined",
			ffExists:            true,
			loggingBinDir:       "loggingBinDir",
			fluentBitExePath:    "fluentBitExePath",
			expectedPathLinux:   "fluentBitExePath",
			expectedPathWindows: "fluentBitExePath",
		},
		{
			name:                "configuration should rule with ff enabled and loggin dir defined",
			ffExists:            true,
			ffEnabled:           true,
			loggingBinDir:       "loggingBinDir",
			fluentBitExePath:    "fluentBitExePath",
			expectedPathLinux:   "fluentBitExePath",
			expectedPathWindows: "fluentBitExePath",
		},
		{
			name:                "loggingBinDir configuration should rule when no fluentBitExePath is present",
			loggingBinDir:       "loggingBinDir",
			integrationsDir:     "integrationsDir",
			expectedPathLinux:   filepath.Join("loggingBinDir", "fluent-bit"),
			expectedPathWindows: filepath.Join("loggingBinDir", "fluent-bit.exe"),
		},
		{
			name:                "loggingBinDir configuration should rule when no fluentBitExePath is present with ff",
			loggingBinDir:       "loggingBinDir",
			integrationsDir:     "integrationsDir",
			ffEnabled:           true,
			ffExists:            true,
			expectedPathLinux:   filepath.Join("loggingBinDir", "fluent-bit"),
			expectedPathWindows: filepath.Join("loggingBinDir", "fluent-bit.exe"),
		},
		{
			name:                "no conf options without ff",
			integrationsDir:     "integrationsDir",
			agentDir:            "some_agent_dir",
			expectedPathLinux:   filepath.Join("/opt/fluent-bit/bin", "fluent-bit"),
			expectedPathWindows: filepath.Join("some_agent_dir", "integrationsDir", "logging", "fluent-bit.exe"),
		},
		{
			name:                "no conf options with ff",
			ffEnabled:           true,
			ffExists:            true,
			integrationsDir:     "integrationsDir",
			agentDir:            "some_agent_dir",
			expectedPathLinux:   filepath.Join("/opt/fluent-bit/bin", "fluent-bit"),
			expectedPathWindows: filepath.Join("some_agent_dir", "integrationsDir", "logging-legacy", "fluent-bit.exe"),
		},
	}

	// create temp directory and set it as default directory to use for temporary files
	tmpDir := t.TempDir()

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
			ffRetriever.ShouldGetFeatureFlag(fflag.FlagFluentBit19, testCase.ffEnabled, testCase.ffExists)

			fbIntCfg := NewFBSupervisorConfig(
				ffRetriever,
				testCase.agentDir,
				testCase.integrationsDir,
				testCase.loggingBinDir,
				testCase.fluentBitExePath,
				testCase.fluentBitNRLibPath,
				testCase.fluentBitParsersPath,
				testCase.fbVerbose,
				tmpDir,
			)

			path := fbIntCfg.getFbPath()
			if runtime.GOOS == "linux" {
				assert.Equal(t, testCase.expectedPathLinux, path)
			}
			if runtime.GOOS == "windows" {
				assert.Equal(t, testCase.expectedPathWindows, path)
			}
		})
	}
}

func Test_buildFbExecutorFailsIfNoFbFiles(t *testing.T) {
	t.Parallel()

	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.ShouldNotGetFeatureFlag(fflag.FlagFluentBit19)

	//nolint:goconst
	agentDir := "agentDir"
	integrationsDir := "integrationsDir"
	loggingBinDir := "loggingBinDir"
	fbVerbose := false
	// create temp directory and set it as default directory to use for temporary files
	tmpDir := t.TempDir()

	fbIntCfg := NewFBSupervisorConfig(
		ffRetriever,
		agentDir,
		integrationsDir,
		loggingBinDir,
		"not existent file",
		"not existent file",
		"not existent file",
		fbVerbose,
		tmpDir,
	)

	executorBuilder := buildFbExecutor(fbIntCfg, confLoaderForTest())
	_, err := executorBuilder()
	require.ErrorIs(t, err, errFbNotAvailable)
}

func Test_buildFbExecutor(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)

	defer func() {
		os.RemoveAll(tmpDir)
	}()

	fluentBitExePath, err := os.CreateTemp(tmpDir, "fb_exe_")
	assert.NoError(t, err)
	fluentBitNRLibPath, err := os.CreateTemp(tmpDir, "fb_lib")
	assert.NoError(t, err)
	fluentBitParsersPath, err := os.CreateTemp(tmpDir, "fb_parser")
	assert.NoError(t, err)

	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.ShouldNotGetFeatureFlag(fflag.FlagFluentBit19)

	agentDir := "agentDir"
	integrationsDir := "agentDir"
	loggingBinDir := "loggingBinDir"
	fbVerbose := false
	// create temp directory and set it as default directory to use for temporary files
	fbTmpDir, err := os.MkdirTemp(tmpDir, "fb_tmp_dir")
	assert.NoError(t, err)

	fbIntCfg := NewFBSupervisorConfig(
		ffRetriever,
		agentDir,
		integrationsDir,
		loggingBinDir,
		fluentBitExePath.Name(),
		fluentBitNRLibPath.Name(),
		fluentBitParsersPath.Name(),
		fbVerbose,
		fbTmpDir,
	)

	executorBuilder := buildFbExecutor(fbIntCfg, confLoaderForTest())

	exec, err := executorBuilder()
	require.NoError(t, err)

	assert.Equal(t, fbIntCfg.getFbPath(), exec.(*executor.Executor).Command)        //nolint:forcetypeassert
	assert.Equal(t, fluentBitNRLibPath.Name(), exec.(*executor.Executor).Args[3])   //nolint:forcetypeassert
	assert.Equal(t, fluentBitParsersPath.Name(), exec.(*executor.Executor).Args[5]) //nolint:forcetypeassert
}

func confLoaderForTest() *logs.CfgLoader {
	agentIdentity := func() entity.Identity {
		return entity.Identity{ID: 13}
	}
	hostnameResolver := testhelpers.NewFakeHostnameResolver("full_hostname", "short_hostname", nil)
	license := "some_license"
	c := config.LogForward{License: license, Troubleshoot: config.Troubleshoot{Enabled: true}}

	return logs.NewFolderLoader(c, agentIdentity, hostnameResolver)
}

// bypassIsLogForwarderAvailable bypasses the check of some files to be able to run fb
// this check was not done before the FF so in some tests this needs to be bypassed.
func bypassIsLogForwarderAvailable(t *testing.T, conf *fBSupervisorConfig) {
	t.Helper()
	// bypass is forwarder available
	file, err := os.CreateTemp("", "")
	assert.NoError(t, err)
	conf.fluentBitExePath = file.Name()
	conf.FluentBitNRLibPath = file.Name()
	conf.FluentBitParsersPath = file.Name()
}
