// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpBackoff(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Duration
		max      time.Duration
		count    uint32
		expected time.Duration
	}{
		{
			name:     "first retry",
			base:     time.Second,
			max:      time.Minute,
			count:    1,
			expected: 2 * time.Second,
		},
		{
			name:     "second retry",
			base:     time.Second,
			max:      time.Minute,
			count:    2,
			expected: 3 * time.Second,
		},
		{
			name:     "fifth retry",
			base:     time.Second,
			max:      time.Minute,
			count:    5,
			expected: 17 * time.Second,
		},
		{
			name:     "exceeds max returns max",
			base:     time.Second,
			max:      10 * time.Second,
			count:    10,
			expected: 10 * time.Second,
		},
		{
			name:     "count at max boundary",
			base:     time.Second,
			max:      time.Hour,
			count:    MaxBackoffErrorCount,
			expected: time.Hour,
		},
		{
			name:     "count exceeds max boundary",
			base:     time.Second,
			max:      time.Hour,
			count:    MaxBackoffErrorCount + 1,
			expected: time.Hour,
		},
		{
			name:     "zero base",
			base:     0,
			max:      time.Minute,
			count:    3,
			expected: 4 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpBackoff(tt.base, tt.max, tt.count)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		envValue    string
		dfault      string
		combineWith []string
		expected    string
		setEnv      bool
	}{
		{
			name:        "env not set returns default",
			key:         "TEST_NOT_SET_VAR",
			envValue:    "",
			dfault:      "default_value",
			combineWith: nil,
			expected:    "default_value",
			setEnv:      false,
		},
		{
			name:        "env set returns value",
			key:         "TEST_SET_VAR",
			envValue:    "env_value",
			dfault:      "default_value",
			combineWith: nil,
			expected:    "env_value",
			setEnv:      true,
		},
		{
			name:        "combine with single path",
			key:         "TEST_PATH_VAR",
			envValue:    "/base",
			dfault:      "/default",
			combineWith: []string{"subdir"},
			expected:    filepath.Join("/base", "subdir"),
			setEnv:      true,
		},
		{
			name:        "combine with multiple paths",
			key:         "TEST_PATHS_VAR",
			envValue:    "/base",
			dfault:      "/default",
			combineWith: []string{"sub1", "sub2", "file.txt"},
			expected:    filepath.Join("/base", "sub1", "sub2", "file.txt"),
			setEnv:      true,
		},
		{
			name:        "combine with default",
			key:         "TEST_COMBINE_DEFAULT",
			envValue:    "",
			dfault:      "/default",
			combineWith: []string{"subdir"},
			expected:    filepath.Join("/default", "subdir"),
			setEnv:      false,
		},
		{
			name:        "empty env value uses default",
			key:         "TEST_EMPTY_VAR",
			envValue:    "",
			dfault:      "default_value",
			combineWith: nil,
			expected:    "default_value",
			setEnv:      true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setEnv {
				t.Setenv(testCase.key, testCase.envValue)
			}

			result := GetEnv(testCase.key, testCase.dfault, testCase.combineWith...)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

func TestFileMD5(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name:        "hash simple content",
			content:     "hello world",
			expectError: false,
		},
		{
			name:        "hash empty file",
			content:     "",
			expectError: false,
		},
		{
			name:        "hash binary content",
			content:     string([]byte{0x00, 0x01, 0x02, 0xFF}),
			expectError: false,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "testfile")
			err := os.WriteFile(tmpFile, []byte(tt.content), 0o600)
			require.NoError(t, err)

			hash, err := FileMD5(tmpFile)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, hash, 16) // MD5 produces 16 bytes
			}
		})
	}
}

func TestFileMD5_FileNotFound(t *testing.T) {
	hash, err := FileMD5("/nonexistent/path/file.txt")
	require.Error(t, err)
	assert.Nil(t, hash)
}

func TestFileMD5_Consistency(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	content := "consistent content"
	err := os.WriteFile(tmpFile, []byte(content), 0o600)
	require.NoError(t, err)

	hash1, err := FileMD5(tmpFile)
	require.NoError(t, err)

	hash2, err := FileMD5(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestMaxBackoffErrorCount(t *testing.T) {
	assert.Equal(t, 31, MaxBackoffErrorCount)
}

func TestSanitizeFileNameCacheSize(t *testing.T) {
	assert.Equal(t, 1000, SanitizeFileNameCacheSize)
}

func TestHiddenField(t *testing.T) {
	assert.Equal(t, "<HIDDEN>", HiddenField)
}

func TestSensitiveKeys(t *testing.T) {
	expectedKeys := []string{"key", "secret", "password", "token", "passphrase", "credential"}
	assert.Equal(t, expectedKeys, SensitiveKeys)
}

func TestJsonFilesRegexp(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{
			name:     "valid json file",
			filename: "config.json",
			expected: true,
		},
		{
			name:     "json with path",
			filename: "/path/to/config.json",
			expected: true,
		},
		{
			name:     "tilde prefix should not match",
			filename: "~config.json",
			expected: false,
		},
		{
			name:     "swap file should not match",
			filename: "config.json.swp",
			expected: false,
		},
		{
			name:     "not json extension",
			filename: "config.yaml",
			expected: false,
		},
		{
			name:     "empty string",
			filename: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JsonFilesRegexp.MatchString(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestISO8601RE(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldMatch bool
	}{
		{
			name:        "UTC timestamp",
			input:       "2015-06-29T16:04:53Z",
			shouldMatch: true,
		},
		{
			name:        "lowercase z",
			input:       "2015-06-29T16:04:53z",
			shouldMatch: true,
		},
		{
			name:        "positive offset",
			input:       "2015-06-29T16:04:53+07:00",
			shouldMatch: true,
		},
		{
			name:        "negative offset",
			input:       "2015-06-29T16:04:53-07:00",
			shouldMatch: true,
		},
		{
			name:        "not a timestamp",
			input:       "hello world",
			shouldMatch: false,
		},
		{
			name:        "partial timestamp",
			input:       "2015-06-29",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ISO8601RE.MatchString(tt.input)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestObfuscateSensitiveDataFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no sensitive data",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "password in command",
			input:    "-password=secret123",
			expected: "-password=<HIDDEN>",
		},
		{
			name:     "token in env var",
			input:    "MY_TOKEN=abc123",
			expected: "MY_TOKEN=<HIDDEN>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ObfuscateSensitiveDataFromString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
