// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	tests := []struct {
		name        string
		srcContent  string
		destExists  bool
		expectError bool
	}{
		{
			name:        "copy to new file",
			srcContent:  "test content",
			destExists:  false,
			expectError: false,
		},
		{
			name:        "overwrite existing file",
			srcContent:  "new content",
			destExists:  true,
			expectError: false,
		},
		{
			name:        "copy empty file",
			srcContent:  "",
			destExists:  false,
			expectError: false,
		},
		{
			name:        "copy large content",
			srcContent:  string(make([]byte, 10000)),
			destExists:  false,
			expectError: false,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			srcFile := filepath.Join(tmpDir, "source.txt")
			destFile := filepath.Join(tmpDir, "dest.txt")

			// Create source file
			err := os.WriteFile(srcFile, []byte(tt.srcContent), 0o600)
			require.NoError(t, err)

			// Create dest file if needed
			if tt.destExists {
				err = os.WriteFile(destFile, []byte("old content"), 0o600)
				require.NoError(t, err)
			}

			// Execute copy
			err = CopyFile(srcFile, destFile)
			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			// Verify content
			destContent, err := os.ReadFile(destFile)
			require.NoError(t, err)
			assert.Equal(t, tt.srcContent, string(destContent))
		})
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "nonexistent.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")

	err := CopyFile(srcFile, destFile)
	require.Error(t, err)
}

func TestCopyFile_DestIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	destDir := filepath.Join(tmpDir, "destdir")

	// Create source file
	err := os.WriteFile(srcFile, []byte("test"), 0o600)
	require.NoError(t, err)

	// Create destination directory
	err = os.Mkdir(destDir, 0o755)
	require.NoError(t, err)

	err = CopyFile(srcFile, destDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot copy file")
}

func TestCopyFile_PreservesPermissions(t *testing.T) {
	if GetOS() == OS_WINDOWS {
		t.Skip("Skipping permission test on Windows")
	}

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.sh")
	destFile := filepath.Join(tmpDir, "dest.sh")

	// Create source file with execute permission
	err := os.WriteFile(srcFile, []byte("#!/bin/bash\necho hello"), 0o700) //nolint:gosec
	require.NoError(t, err)

	err = CopyFile(srcFile, destFile)
	require.NoError(t, err)

	// Verify permissions preserved
	srcInfo, err := os.Stat(srcFile)
	require.NoError(t, err)
	destInfo, err := os.Stat(destFile)
	require.NoError(t, err)

	assert.Equal(t, srcInfo.Mode().Perm(), destInfo.Mode().Perm())
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		setup    func() string
		expected bool
	}{
		{
			name: "existing file",
			setup: func() string {
				path := filepath.Join(tmpDir, "exists.txt")
				_ = os.WriteFile(path, []byte("test"), 0o600)

				return path
			},
			expected: true,
		},
		{
			name: "nonexistent file",
			setup: func() string {
				return filepath.Join(tmpDir, "nonexistent.txt")
			},
			expected: false,
		},
		{
			name: "existing directory",
			setup: func() string {
				path := filepath.Join(tmpDir, "existsdir")
				_ = os.Mkdir(path, 0o755)

				return path
			},
			expected: true,
		},
		{
			name: "empty filename",
			setup: func() string {
				return ""
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			result := FileExists(path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileExists_SymbolicLink(t *testing.T) {
	if GetOS() == OS_WINDOWS {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.txt")
	symlink := filepath.Join(tmpDir, "symlink.txt")

	// Create real file
	err := os.WriteFile(realFile, []byte("test"), 0o600)
	require.NoError(t, err)

	// Create symlink
	err = os.Symlink(realFile, symlink)
	require.NoError(t, err)

	assert.True(t, FileExists(symlink))

	// Remove real file - broken symlink
	err = os.Remove(realFile)
	require.NoError(t, err)

	// Broken symlink should still return false (Stat follows symlinks)
	assert.False(t, FileExists(symlink))
}
