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

func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile.txt")
	content := []byte("test content")

	err := WriteFile(testFile, content, 0o644)
	require.NoError(t, err)

	// Verify file was created with correct content
	readContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, content, readContent)
}

func TestOpenFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "openfile.txt")

	// Create file
	f, err := OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	assert.NotNil(t, f)
	f.Close()

	// Verify file exists
	_, err = os.Stat(testFile)
	require.NoError(t, err)
}

func TestCreate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "created.txt")

	f, err := Create(testFile)
	require.NoError(t, err)
	assert.NotNil(t, f)
	f.Close()

	// Verify file exists
	_, err = os.Stat(testFile)
	require.NoError(t, err)
}

func TestMkdirAll(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "level1", "level2", "level3")

	err := MkdirAll(nestedDir, 0o755)
	require.NoError(t, err)

	// Verify directory exists
	info, err := os.Stat(nestedDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestWriteFile_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	err := WriteFile(testFile, []byte{}, 0o644)
	require.NoError(t, err)

	// Verify file was created
	info, err := os.Stat(testFile)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestOpenFile_ReadOnly(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "readonly.txt")

	// Create file first
	err := WriteFile(testFile, []byte("test"), 0o644)
	require.NoError(t, err)

	// Open for reading
	f, err := OpenFile(testFile, os.O_RDONLY, 0)
	require.NoError(t, err)
	assert.NotNil(t, f)
	f.Close()
}
