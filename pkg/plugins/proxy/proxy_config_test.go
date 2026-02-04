// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUrlEntry(t *testing.T) {
	tests := []struct {
		name           string
		rawURL         string
		expectedScheme string
		expectedError  string
		isNil          bool
	}{
		{
			name:           "https url",
			rawURL:         "https://proxy.example.com:8080",
			expectedScheme: "https",
			expectedError:  "",
			isNil:          false,
		},
		{
			name:           "http url",
			rawURL:         "http://proxy.example.com:8080",
			expectedScheme: "http",
			expectedError:  "",
			isNil:          false,
		},
		{
			name:           "socks5 url",
			rawURL:         "socks5://proxy.example.com:1080",
			expectedScheme: "socks5",
			expectedError:  "",
			isNil:          false,
		},
		{
			name:           "empty url returns nil",
			rawURL:         "",
			expectedScheme: "",
			expectedError:  "",
			isNil:          true,
		},
		{
			name:           "url with auth",
			rawURL:         "http://user:pass@proxy.example.com:8080",
			expectedScheme: "http",
			expectedError:  "",
			isNil:          false,
		},
		{
			name:           "url with path",
			rawURL:         "http://proxy.example.com:8080/path",
			expectedScheme: "http",
			expectedError:  "",
			isNil:          false,
		},
		{
			name:           "invalid url",
			rawURL:         "://invalid",
			expectedScheme: "",
			expectedError:  "wrong url",
			isNil:          false,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			result := urlEntry(tt.rawURL)
			if tt.isNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedScheme, result.Scheme)
				assert.Equal(t, tt.expectedError, result.Error)
			}
		})
	}
}

func TestPathEntry(t *testing.T) {
	// Create temp directory and file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "ca-bundle.crt")
	err := os.WriteFile(tmpFile, []byte("test"), 0o600)
	require.NoError(t, err)

	tests := []struct {
		name         string
		path         string
		expectedType string
		isNil        bool
	}{
		{
			name:         "empty path returns nil",
			path:         "",
			expectedType: "",
			isNil:        true,
		},
		{
			name:         "directory path",
			path:         tmpDir,
			expectedType: typeDir,
			isNil:        false,
		},
		{
			name:         "file path",
			path:         tmpFile,
			expectedType: typeFile,
			isNil:        false,
		},
		{
			name:         "nonexistent path",
			path:         "/nonexistent/path",
			expectedType: "unexpected error",
			isNil:        false,
		},
	}

	for _, tt := range tests { //nolint:varnamelen
		t.Run(tt.name, func(t *testing.T) {
			result := pathEntry(tt.path)
			if tt.isNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedType, result.Type)
			}
		})
	}
}

func TestEntry_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		entry    entry
		expected string
	}{
		{
			name:     "standard id",
			entry:    entry{Id: "proxy"},
			expected: "proxy",
		},
		{
			name:     "empty id",
			entry:    entry{Id: ""},
			expected: "",
		},
		{
			name:     "id with special characters",
			entry:    entry{Id: "ca_bundle_file"},
			expected: "ca_bundle_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.SortKey())
		})
	}
}

func TestProxyEntry_Fields(t *testing.T) {
	proxyE := proxyEntry{
		entry:  entry{Id: "test_proxy"},
		Scheme: "https",
		Error:  "",
	}

	assert.Equal(t, "test_proxy", proxyE.Id)
	assert.Equal(t, "https", proxyE.Scheme)
	assert.Empty(t, proxyE.Error)
}

func TestFileEntry_Fields(t *testing.T) {
	fileE := fileEntry{
		entry: entry{Id: "ca_bundle"},
		Type:  typeFile,
	}

	assert.Equal(t, "ca_bundle", fileE.Id)
	assert.Equal(t, typeFile, fileE.Type)
}

func TestBoolEntry_Fields(t *testing.T) {
	tests := []struct {
		name     string
		entry    boolEntry
		expected bool
	}{
		{
			name: "true value",
			entry: boolEntry{
				entry: entry{Id: "ignore_proxy"},
				Value: true,
			},
			expected: true,
		},
		{
			name: "false value",
			entry: boolEntry{
				entry: entry{Id: "validate_certs"},
				Value: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.Value)
		})
	}
}

func TestTypeConstants(t *testing.T) {
	assert.Equal(t, "file", typeFile)
	assert.Equal(t, "directory", typeDir)
}
