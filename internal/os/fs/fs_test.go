// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFirstLine(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    string
		expectError bool
	}{
		{
			name:        "single line",
			content:     "Hello World",
			expected:    "Hello World",
			expectError: false,
		},
		{
			name:        "multiple lines returns first",
			content:     "First Line\nSecond Line\nThird Line",
			expected:    "First Line",
			expectError: false,
		},
		{
			name:        "trims whitespace",
			content:     "  Trimmed Content  \nSecond Line",
			expected:    "Trimmed Content",
			expectError: false,
		},
		{
			name:        "empty file",
			content:     "",
			expected:    "",
			expectError: false,
		},
		{
			name:        "only newline",
			content:     "\n",
			expected:    "",
			expectError: false,
		},
		{
			name:        "tabs and spaces",
			content:     "\t  Hello  \t\nWorld",
			expected:    "Hello",
			expectError: false,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			result, err := ReadFirstLine(tmpFile)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestReadFirstLine_FileNotFound(t *testing.T) {
	result, err := ReadFirstLine("/nonexistent/path/file.txt")
	require.Error(t, err)
	assert.Equal(t, "unknown", result)
	assert.Contains(t, err.Error(), "cannot open file")
}

func TestReadFileFieldMatching(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		pattern     string
		expected    string
		expectError bool
	}{
		{
			name:        "match on first line",
			content:     "VERSION=1.2.3\nNAME=test",
			pattern:     `VERSION=(.+)`,
			expected:    "1.2.3",
			expectError: false,
		},
		{
			name:        "match on second line",
			content:     "NAME=test\nVERSION=1.2.3",
			pattern:     `VERSION=(.+)`,
			expected:    "1.2.3",
			expectError: false,
		},
		{
			name:        "match with quotes",
			content:     `NAME="Ubuntu"\nVERSION="20.04"`,
			pattern:     `NAME="([^"]+)"`,
			expected:    "Ubuntu",
			expectError: false,
		},
		{
			name:        "no match returns unknown",
			content:     "FOO=bar\nBAZ=qux",
			pattern:     `VERSION=(.+)`,
			expected:    "unknown",
			expectError: false,
		},
		{
			name:        "complex regex",
			content:     "Red Hat Enterprise Linux release 8.4 (Ootpa)",
			pattern:     `release (\d+\.\d+)`,
			expected:    "8.4",
			expectError: false,
		},
		{
			name:        "empty file returns unknown",
			content:     "",
			pattern:     `(.+)`,
			expected:    "unknown",
			expectError: false,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)

			defer os.Remove(tmpFile)

			re := regexp.MustCompile(tt.pattern)

			result, err := ReadFileFieldMatching(tmpFile, re)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				// Note: err may still be nil even when result is "unknown"
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestReadFileFieldMatching_FileNotFound(t *testing.T) {
	re := regexp.MustCompile(`(.+)`)
	result, err := ReadFileFieldMatching("/nonexistent/path/file.txt", re)
	require.Error(t, err)
	assert.Equal(t, "unknown", result)
	assert.Contains(t, err.Error(), "cannot open file")
}

func TestReadFileFieldMatching_MultipleCaptures(t *testing.T) {
	content := "VERSION_ID=8.4"

	tmpFile := createTempFile(t, content)
	defer os.Remove(tmpFile)

	// Pattern with multiple capture groups - should return first capture
	re := regexp.MustCompile(`VERSION_ID=(\d+)\.(\d+)`)
	result, err := ReadFileFieldMatching(tmpFile, re)
	require.NoError(t, err)
	assert.Equal(t, "8", result) // First capture group
}

func TestReadFileFieldMatching_LinuxReleaseFiles(t *testing.T) {
	t.Run("os-release NAME", func(t *testing.T) {
		tmpFile := createTempFile(t, `NAME="Ubuntu"\nVERSION="20.04.3 LTS (Focal Fossa)"`)
		defer os.Remove(tmpFile)

		re := regexp.MustCompile(`NAME="?([^"\n]+)"?`)
		result, err := ReadFileFieldMatching(tmpFile, re)
		require.NoError(t, err)
		assert.Equal(t, "Ubuntu", result)
	})

	t.Run("centos-release", func(t *testing.T) {
		tmpFile := createTempFile(t, "CentOS Linux release 7.9.2009 (Core)")
		defer os.Remove(tmpFile)

		re := regexp.MustCompile(`release (\d+\.\d+)`)
		result, err := ReadFileFieldMatching(tmpFile, re)
		require.NoError(t, err)
		assert.Equal(t, "7.9", result)
	})

	t.Run("redhat-release", func(t *testing.T) {
		tmpFile := createTempFile(t, "Red Hat Enterprise Linux release 8.4 (Ootpa)")
		defer os.Remove(tmpFile)

		re := regexp.MustCompile(`release (\d+\.\d+)`)
		result, err := ReadFileFieldMatching(tmpFile, re)
		require.NoError(t, err)
		assert.Equal(t, "8.4", result)
	})
}

// createTempFile creates a temporary file with the given content and returns its path.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	require.NoError(t, err)

	return tmpFile
}
