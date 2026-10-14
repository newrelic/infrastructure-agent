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
		classic bool
	}{
		{FBVersion{1, 9, 3}, false, true},
		{FBVersion{3, 1, 9}, false, true},
		{FBVersion{3, 2, 0}, true, true},
		{FBVersion{5, 1, 2}, true, true},
		{FBVersion{5, 9, 0}, true, true},
		{FBVersion{6, 0, 0}, true, false},
		{FBVersion{6, 1, 0}, true, false},
		{FBVersion{7, 0, 0}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.version.String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.yaml, tc.version.supportsYAML())
			assert.Equal(t, tc.classic, tc.version.supportsClassic())
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
	v59 := FBVersion{5, 9, 0}
	v60 := FBVersion{6, 0, 0}

	cases := []struct {
		name       string
		version    FBVersion
		versionErr error
		external   logs.FBCfgExternal
		expected   logs.FBConfigFormat
		wantErr    error
	}{
		// no external config
		{"1.9 classic", v19, nil, noExternal, logs.FBConfigFormatClassic, nil},
		{"5.1 classic", v51, nil, noExternal, logs.FBConfigFormatClassic, nil},
		{"5.9 classic", v59, nil, noExternal, logs.FBConfigFormatClassic, nil},
		{"6.0 yaml", v60, nil, noExternal, logs.FBConfigFormatYAML, nil},
		{"unknown classic", FBVersion{}, errUnknown, noExternal, logs.FBConfigFormatClassic, nil},
		// external .conf
		{"5.1 external conf", v51, nil, externalConf, logs.FBConfigFormatClassic, nil},
		{"6.0 external conf", v60, nil, externalConf, "", errFbExternalCfgMismatch},
		{"unknown external conf", FBVersion{}, errUnknown, externalConf, logs.FBConfigFormatClassic, nil},
		// external .yaml
		{"5.1 external yaml", v51, nil, externalYAML, logs.FBConfigFormatYAML, nil},
		{"6.0 external yaml", v60, nil, externalYAML, logs.FBConfigFormatYAML, nil},
		{"1.9 external yaml", v19, nil, externalYAML, "", errFbYAMLNotSupported},
		{"unknown external yaml", FBVersion{}, errUnknown, externalYAML, logs.FBConfigFormatYAML, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			format, err := selectFBConfigFormat(tc.version, tc.versionErr, tc.external)
			assert.Equal(t, tc.expected, format)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
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
	output = "Fluent Bit v6.0.0\n"
	require.NoError(t, os.WriteFile(exePath, []byte("v2 binary"), 0o600))
	require.NoError(t, os.Chtimes(exePath, time.Now(), time.Now().Add(time.Minute)))

	version, err = detector.Detect(exePath)
	require.NoError(t, err)
	assert.Equal(t, FBVersion{6, 0, 0}, version)
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
