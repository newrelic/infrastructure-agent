// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package disk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMkdirAll_CreatesFreshDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "fresh")

	require.NoError(t, MkdirAll(target, 0o700))

	pathInfo, err := os.Lstat(target)
	require.NoError(t, err)
	assert.True(t, pathInfo.IsDir())
	assert.Equal(t, os.FileMode(0o700), pathInfo.Mode().Perm())
}

func TestMkdirAll_LeavesSafeExistingDirUntouched(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "existing")
	require.NoError(t, os.Mkdir(target, 0o700))

	marker := filepath.Join(target, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("keep me"), 0o600))

	require.NoError(t, MkdirAll(target, 0o700))

	content, err := os.ReadFile(marker)
	require.NoError(t, err, "marker file should survive a call over a safe, already-owned directory")
	assert.Equal(t, "keep me", string(content))
}

func TestMkdirAll_ReplacesSymlink(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	realDir := filepath.Join(base, "real-target")
	require.NoError(t, os.Mkdir(realDir, 0o700))

	target := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(realDir, target))

	require.NoError(t, MkdirAll(target, 0o700))

	pathInfo, err := os.Lstat(target)
	require.NoError(t, err)
	assert.Zero(t, pathInfo.Mode()&os.ModeSymlink, "symlink should have been replaced with a real directory")
	assert.True(t, pathInfo.IsDir())
}

func TestMkdirAll_ReplacesGroupOtherWritableDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "loose")
	require.NoError(t, os.Mkdir(target, 0o777))
	require.NoError(t, os.Chmod(target, 0o777)) // bypass umask to guarantee group/other write bits

	marker := filepath.Join(target, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("should be gone"), 0o600))

	require.NoError(t, MkdirAll(target, 0o700))

	pathInfo, err := os.Lstat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), pathInfo.Mode().Perm())

	_, err = os.Stat(marker)
	assert.True(t, os.IsNotExist(err), "unsafe directory should have been wiped, not reused")
}
