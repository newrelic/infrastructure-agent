// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package initialize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveAllClassic(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		build func(t *testing.T, root string)
	}{
		{
			name: "files only",
			build: func(t *testing.T, root string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o600))
			},
		},
		{
			name: "nested subdirectory",
			build: func(t *testing.T, root string) {
				t.Helper()
				sub := filepath.Join(root, "sub")
				require.NoError(t, os.MkdirAll(sub, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(sub, "c.txt"), []byte("c"), 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o600))
			},
		},
		{
			name: "deeply nested subdirectories",
			build: func(t *testing.T, root string) {
				t.Helper()
				deep := filepath.Join(root, "sub1", "sub2")
				require.NoError(t, os.MkdirAll(deep, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(deep, "d.txt"), []byte("d"), 0o600))
			},
		},
		{
			name: "empty directory",
			build: func(t *testing.T, _ string) {
				t.Helper()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "victim")
			require.NoError(t, os.MkdirAll(root, 0o755))
			tc.build(t, root)

			err := removeAllClassic(root)
			require.NoError(t, err)

			_, statErr := os.Stat(root)
			assert.True(t, os.IsNotExist(statErr), "expected %s to be removed", root)
		})
	}
}

func TestRemoveAllClassic_NonExistentPathReturnsNil(t *testing.T) {
	t.Parallel()

	err := removeAllClassic(filepath.Join(t.TempDir(), "does-not-exist"))
	assert.NoError(t, err)
}

func TestRemoveAllClassic_PropagatesErrorFromNestedFile(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "victim")
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	locked := filepath.Join(sub, "locked.txt")
	require.NoError(t, os.WriteFile(locked, []byte("locked"), 0o600))
	require.NoError(t, os.Chmod(locked, 0o400))

	t.Cleanup(func() {
		_ = os.Chmod(locked, 0o600)
	})

	err := removeAllClassic(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove path")

	_, statErr := os.Stat(locked)
	assert.NoError(t, statErr, "expected locked file to survive the failed removal")
}

func TestRemoveAllWithFallback_DeletesNormally(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "a.txt"), []byte("a"), 0o600))

	err := removeAllWithFallback(root)
	require.NoError(t, err)

	_, statErr := os.Stat(root)
	assert.True(t, os.IsNotExist(statErr))
}
