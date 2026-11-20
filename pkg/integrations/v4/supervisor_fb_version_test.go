// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	ctx2 "context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFBVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		output   string
		expected FBVersion
		wantErr  bool
	}{
		{"td-agent-bit 1.9", "Fluent Bit v1.9.3\nGit commit: 1234\n", FBVersion{1, 9, 3}, false},
		{"2.0", "Fluent Bit v2.0.8\nGit commit: 1234\n", FBVersion{2, 0, 8}, false},
		{"3.2", "Fluent Bit v3.2.10\nGit commit: 1234\n", FBVersion{3, 2, 10}, false},
		{"5.1", "Fluent Bit v5.1.2\nGit commit: 1234\n", FBVersion{5, 1, 2}, false},
		{"windows line endings", "Fluent Bit v5.0.8\r\nGit commit: 1234\r\n", FBVersion{5, 0, 8}, false},
		{"garbage", "command not found", FBVersion{}, true},
		{"empty", "", FBVersion{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			version, err := ParseFBVersion(tc.output)
			assert.Equal(t, tc.expected, version)
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestFBVersion_Support(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version FBVersion
		yaml    bool
	}{
		{FBVersion{1, 9, 3}, false},
		{FBVersion{3, 1, 9}, false},
		{FBVersion{3, 2, 0}, true},
		{FBVersion{5, 1, 2}, true},
		{FBVersion{5, 0, 0}, true},
	}
	for _, tc := range cases {
		t.Run(tc.version.String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.yaml, tc.version.supportsYAML())
		})
	}
}

func TestSelectFBConfigFormat(t *testing.T) {
	t.Parallel()

	errUnknown := errors.New("unknown")
	noExternal := logs.FBCfgExternal{}                                     //nolint:exhaustruct
	externalConf := logs.FBCfgExternal{CfgFilePath: "/etc/fb/custom.conf"} //nolint:exhaustruct
	externalYAML := logs.FBCfgExternal{CfgFilePath: "/etc/fb/custom.yaml"} //nolint:exhaustruct

	v19 := FBVersion{1, 9, 3}
	v51 := FBVersion{5, 1, 2}
	v50 := FBVersion{5, 0, 0}

	cases := []struct {
		name       string
		version    FBVersion
		versionErr error
		external   logs.FBCfgExternal
		yamlFlag   bool
		expected   logs.FBConfigFormat
		wantErr    error
	}{
		// no external config
		{"1.9 classic", v19, nil, noExternal, false, logs.FBConfigFormatClassic, nil},
		{"5.1 classic", v51, nil, noExternal, false, logs.FBConfigFormatClassic, nil},
		{"5.0 classic when flag disabled", v50, nil, noExternal, false, logs.FBConfigFormatClassic, nil},
		{"unknown classic", FBVersion{}, errUnknown, noExternal, false, logs.FBConfigFormatClassic, nil},
		// external .conf
		{"5.1 external conf", v51, nil, externalConf, false, logs.FBConfigFormatClassic, nil},
		{"5.0 external conf", v50, nil, externalConf, false, logs.FBConfigFormatClassic, nil},
		{"unknown external conf", FBVersion{}, errUnknown, externalConf, false, logs.FBConfigFormatClassic, nil},
		// external .yaml
		{"5.1 external yaml", v51, nil, externalYAML, true, logs.FBConfigFormatYAML, nil},
		{"5.0 external yaml", v50, nil, externalYAML, true, logs.FBConfigFormatYAML, nil},
		{"1.9 external yaml", v19, nil, externalYAML, true, "", errFbYAMLNotSupported},
		{"unknown external yaml", FBVersion{}, errUnknown, externalYAML, true, logs.FBConfigFormatYAML, nil},
		// yaml feature flag enabled
		{"flag 1.9 stays classic", v19, nil, noExternal, true, logs.FBConfigFormatClassic, nil},
		{"flag 3.1 stays classic", FBVersion{3, 1, 9}, nil, noExternal, true, logs.FBConfigFormatClassic, nil},
		{"flag 3.2 yaml", FBVersion{3, 2, 10}, nil, noExternal, true, logs.FBConfigFormatYAML, nil},
		{"flag 5.1 yaml", v51, nil, noExternal, true, logs.FBConfigFormatYAML, nil},
		{"flag 5.0 yaml", v50, nil, noExternal, true, logs.FBConfigFormatYAML, nil},
		{"flag unknown classic", FBVersion{}, errUnknown, noExternal, true, logs.FBConfigFormatClassic, nil},
		{"flag 5.1 external conf stays classic", v51, nil, externalConf, true, logs.FBConfigFormatClassic, nil},
		{"flag 5.0 external conf stays classic", v50, nil, externalConf, true, logs.FBConfigFormatClassic, nil},
		{"flag unknown external conf", FBVersion{}, errUnknown, externalConf, true, logs.FBConfigFormatClassic, nil},
		{"flag 5.1 external yaml", v51, nil, externalYAML, true, logs.FBConfigFormatYAML, nil},
		{"flag 1.9 external conf stays classic", v19, nil, externalConf, true, logs.FBConfigFormatClassic, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			format, err := selectFBConfigFormat(tc.version, tc.versionErr, tc.external, "", tc.yamlFlag)
			assert.Equal(t, tc.expected, format)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

func TestSelectFBConfigFormat_UserSetting(t *testing.T) {
	t.Parallel()

	errUnknown := errors.New("unknown")
	none := logs.FBCfgExternal{}                                      //nolint:exhaustruct
	extConf := logs.FBCfgExternal{CfgFilePath: "/etc/fb/custom.conf"} //nolint:exhaustruct
	extYAML := logs.FBCfgExternal{CfgFilePath: "/etc/fb/custom.yaml"} //nolint:exhaustruct
	v19, v51, unknown := FBVersion{1, 9, 3}, FBVersion{5, 1, 2}, FBVersion{}

	cases := []struct {
		name       string
		version    FBVersion
		versionErr error
		external   logs.FBCfgExternal
		setting    string
		flag       bool
		expected   logs.FBConfigFormat
		wantErr    error
	}{
		{"yaml needs the flag", v51, nil, none, "yaml", false, logs.FBConfigFormatClassic, nil},
		{"yaml with flag on", v51, nil, none, "yaml", true, logs.FBConfigFormatYAML, nil},
		{"conf wins over flag on", v51, nil, none, "conf", true, logs.FBConfigFormatClassic, nil},
		{"yaml on old version falls back", v19, nil, none, "yaml", true, logs.FBConfigFormatClassic, nil},
		{"yaml on unknown version falls back", unknown, errUnknown, none, "yaml", true, logs.FBConfigFormatClassic, nil},
		{"yaml with external yaml", v51, nil, extYAML, "yaml", true, logs.FBConfigFormatYAML, nil},
		{"yaml with external conf stays classic", v51, nil, extConf, "yaml", true, logs.FBConfigFormatClassic, nil},
		{"yaml with external conf and flag on stays classic", v51, nil, extConf, "yaml", true,
			logs.FBConfigFormatClassic, nil},
		{"yaml with external conf unknown version stays classic", unknown, errUnknown, extConf, "yaml", true,
			logs.FBConfigFormatClassic, nil},
		{"yaml with external yaml unknown version", unknown, errUnknown, extYAML, "yaml", true,
			logs.FBConfigFormatYAML, nil},
		{"conf with external yaml unknown version errors", unknown, errUnknown, extYAML, "conf", true,
			"", errFbConfFormatVsYAMLExternal},
		{"conf with external conf", v51, nil, extConf, "conf", true, logs.FBConfigFormatClassic, nil},
		{"conf with external yaml errors", v51, nil, extYAML, "conf", true, "", errFbConfFormatVsYAMLExternal},
		{"yaml with old version and external yaml errors", v19, nil, extYAML, "yaml", true, "", errFbYAMLNotSupported},
		{"conf with old version and external yaml errors", v19, nil, extYAML, "conf", true, "", errFbYAMLNotSupported},
		{"yaml with old version and external conf", v19, nil, extConf, "yaml", true, logs.FBConfigFormatClassic, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			format, err := selectFBConfigFormat(tc.version, tc.versionErr, tc.external, tc.setting, tc.flag)
			assert.Equal(t, tc.expected, format)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

func TestSelectFBConfigFormat_FlagOffIsAlwaysClassic(t *testing.T) {
	logger, hook := logtest.NewNullLogger()
	originalLogger := sFBLogger
	sFBLogger = func() *logrus.Entry { return logrus.NewEntry(logger) }
	t.Cleanup(func() { sFBLogger = originalLogger })

	versions := []struct {
		version FBVersion
		err     error
	}{{FBVersion{1, 9, 3}, nil}, {FBVersion{3, 2, 0}, nil}, {FBVersion{5, 1, 2}, nil}, {FBVersion{}, errors.New("x")}}
	externals := []logs.FBCfgExternal{
		{}, {CfgFilePath: "/etc/fb/custom.conf"}, {CfgFilePath: "/etc/fb/custom.yaml"}, //nolint:exhaustruct
	}

	for _, v := range versions {
		for _, ext := range externals {
			for _, setting := range []string{"", "yaml", "conf"} {
				format, err := selectFBConfigFormat(v.version, v.err, ext, setting, false)
				require.NoError(t, err, "%v %v %q", v.version, ext.CfgFilePath, setting)
				assert.Equal(t, logs.FBConfigFormatClassic, format, "%v %v %q", v.version, ext.CfgFilePath, setting)
			}
		}
	}
	assert.Empty(t, hook.AllEntries(), "flag off must not log anything new")
}

func TestSelectFBConfigFormat_Warnings(t *testing.T) {
	errUnknown := errors.New("unknown")
	none := logs.FBCfgExternal{}                                      //nolint:exhaustruct
	extConf := logs.FBCfgExternal{CfgFilePath: "/etc/fb/custom.conf"} //nolint:exhaustruct
	extYAML := logs.FBCfgExternal{CfgFilePath: "/etc/fb/custom.yaml"} //nolint:exhaustruct
	v19, v51 := FBVersion{1, 9, 3}, FBVersion{5, 1, 2}

	// each warning is matched by a fragment of its message, in log order
	cases := []struct {
		name     string
		version  FBVersion
		verr     error
		external logs.FBCfgExternal
		setting  string
		flag     bool
		warnings []string
	}{
		{"external conf asks to migrate", v51, nil, extConf, "", true,
			[]string{"Newer Fluent Bit features may require YAML"}},
		{"flag off stays quiet with external conf", v51, nil, extConf, "", false, nil},
		{"flag off stays quiet with yaml setting", v19, nil, extConf, "yaml", false, nil},
		{"external conf with setting conf asks to migrate", v51, nil, extConf, "conf", true,
			[]string{"Newer Fluent Bit features may require YAML"}},
		{"external conf with setting yaml explains it is ignored", v51, nil, extConf, "yaml", true,
			[]string{"external config file is classic", "Newer Fluent Bit features may require YAML"}},
		{"external yaml on unknown version warns", FBVersion{}, errUnknown, extYAML, "", true,
			[]string{errFbExternalYAMLNoVersion.Error()}},
		{"yaml setting on old version warns", v19, nil, none, "yaml", true,
			[]string{"does not support it"}},
		{"yaml setting on unknown version warns", FBVersion{}, errUnknown, none, "yaml", true,
			[]string{"could not be detected"}},
		{"external conf on old version stays quiet", v19, nil, extConf, "", true, nil},
		{"no external, defaults, stays quiet", v51, nil, none, "", true, nil},
		{"no external, yaml setting, stays quiet", v51, nil, none, "yaml", true, nil},
		{"no external, conf setting, stays quiet", v51, nil, none, "conf", true, nil},
		{"external yaml, known version, stays quiet", v51, nil, extYAML, "", true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger, hook := logtest.NewNullLogger()
			originalLogger := sFBLogger
			sFBLogger = func() *logrus.Entry { return logrus.NewEntry(logger) }
			t.Cleanup(func() { sFBLogger = originalLogger })

			_, err := selectFBConfigFormat(tc.version, tc.verr, tc.external, tc.setting, tc.flag)
			require.NoError(t, err)

			entries := hook.AllEntries()
			require.Len(t, entries, len(tc.warnings), "unexpected warnings: %v", entries)
			for i, fragment := range tc.warnings {
				assert.Equal(t, logrus.WarnLevel, entries[i].Level)
				assert.Contains(t, entries[i].Message, fragment)
			}
		})
	}
}

func TestSelectFBConfigFormat_ErrorsNameTheFile(t *testing.T) {
	t.Parallel()

	extYAML := logs.FBCfgExternal{CfgFilePath: "/etc/fb/custom.yaml"} //nolint:exhaustruct

	_, err := selectFBConfigFormat(FBVersion{5, 1, 2}, nil, extYAML, "conf", true)
	require.ErrorIs(t, err, errFbConfFormatVsYAMLExternal)
	assert.Contains(t, err.Error(), extYAML.CfgFilePath)
	assert.Contains(t, err.Error(), fbMigrationDocURL)

	_, err = selectFBConfigFormat(FBVersion{1, 9, 3}, nil, extYAML, "", true)
	require.ErrorIs(t, err, errFbYAMLNotSupported)
	assert.Contains(t, err.Error(), "v1.9.3")
}

func TestFBVersionDetector_CachesUntilBinaryChanges(t *testing.T) {
	t.Parallel()

	exePath := filepath.Join(t.TempDir(), "fluent-bit")
	require.NoError(t, os.WriteFile(exePath, []byte("v1"), 0o600))

	calls := 0
	output := "Fluent Bit v5.1.2\n"
	detector := newFBVersionDetector()
	detector.runVersion = func(_ ctx2.Context, _ string) ([]byte, error) {
		calls++

		return []byte(output), nil
	}

	version, err := detector.Detect(exePath)
	require.NoError(t, err)
	assert.Equal(t, FBVersion{5, 1, 2}, version)

	_, err = detector.Detect(exePath)
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "version should be cached while the binary is unchanged")

	// simulate a package upgrade replacing the binary
	output = "Fluent Bit v5.0.0\n"
	require.NoError(t, os.WriteFile(exePath, []byte("v2 binary"), 0o600))
	require.NoError(t, os.Chtimes(exePath, time.Now(), time.Now().Add(time.Minute)))

	version, err = detector.Detect(exePath)
	require.NoError(t, err)
	assert.Equal(t, FBVersion{5, 0, 0}, version)
	assert.Equal(t, 2, calls)
}

func TestFBVersionDetector_Errors(t *testing.T) {
	t.Parallel()

	exePath := filepath.Join(t.TempDir(), "fluent-bit")
	require.NoError(t, os.WriteFile(exePath, []byte("bin"), 0o600))

	errRun := errors.New("exec format error")
	detector := newFBVersionDetector()
	detector.runVersion = func(_ ctx2.Context, _ string) ([]byte, error) { return nil, errRun }

	_, err := detector.Detect(exePath)
	require.ErrorIs(t, err, errRun)

	_, err = detector.Detect(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)

	detector.runVersion = func(_ ctx2.Context, _ string) ([]byte, error) { return []byte("unexpected"), nil }
	_, err = detector.Detect(exePath)
	require.ErrorIs(t, err, errFbVersionNotFound)
}
