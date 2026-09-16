// Copyright 2026 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin

package disk

import (
	"os"
	"path/filepath"
	"syscall"
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

// TestMkdirAll_ReusesWorldWritableMountPoint reproduces
// https://github.com/newrelic/infrastructure-agent/issues/2324: a Kubernetes emptyDir is
// mounted mode 0777 directly at the data dir path. Without the mount-point exemption this
// directory is wrongly flagged as "unsafe" and wiped, which fails outright when the mount is
// the root of a read-only-root-filesystem container (the mount point can't be unlinked from
// its read-only parent) - exactly the fatal "refusing to reuse unsafe path ... read-only file
// system" crash-loop reported in the issue.
//
// It does not call t.Parallel(): it overrides the package-level statDev seam, which must not
// run concurrently with other tests that call MkdirAll/isSafeExistingDir.
//
//nolint:paralleltest
func TestMkdirAll_ReusesWorldWritableMountPoint(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "emptydir")
	require.NoError(t, os.Mkdir(target, 0o777))
	require.NoError(t, os.Chmod(target, 0o777))

	marker := filepath.Join(target, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("keep me"), 0o600))

	restore := fakeMountPoint(t, target)
	defer restore()

	require.NoError(t, MkdirAll(target, 0o700))

	content, err := os.ReadFile(marker)
	require.NoError(t, err, "a distinct mount point should be reused, not wiped, even if world-writable")
	assert.Equal(t, "keep me", string(content))
}

// TestMkdirAll_UnsafeNonMountPointStillReplaced guards against over-widening the exemption:
// a world-writable directory that is NOT a distinct mount point (statDev reports the same
// device as its parent, the common case for a directory planted under a shared writable
// parent like /tmp) must still be treated as unsafe and wiped.
func TestMkdirAll_UnsafeNonMountPointStillReplaced(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "same-device")
	require.NoError(t, os.Mkdir(target, 0o777))
	require.NoError(t, os.Chmod(target, 0o777))

	marker := filepath.Join(target, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("should be gone"), 0o600))

	require.NoError(t, MkdirAll(target, 0o700))

	_, err := os.Stat(marker)
	assert.True(t, os.IsNotExist(err), "same-device writable directory should still be wiped")
}

// TestMkdirAll_ReusesGroupOwnedMountPoint reproduces
// https://github.com/newrelic/infrastructure-agent/issues/2333: a Kubernetes emptyDir is
// chowned to a group (an fsGroup) that the agent's non-root user belongs to via a
// supplementary group, but Kubernetes never changes the directory's *user* ownership away
// from root. Without the group check, the ownership branch always rejects the directory
// before the permission/mount-point check is ever reached, and MkdirAll tries to remove and
// recreate it - which fails outright when it's the root of a read-only-root-filesystem mount.
//
// The directory is actually created (and thus owned) by the test process itself; currentUID
// is faked to a different value so the uid branch is forced to fail and fall through to the
// group check, which then matches against the test process's real, unmodified gid.
//
// It does not call t.Parallel(): it overrides the package-level currentUID and statDev seams,
// which must not run concurrently with other tests that call MkdirAll/isSafeExistingDir.
//
//nolint:paralleltest
func TestMkdirAll_ReusesGroupOwnedMountPoint(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "emptydir")
	require.NoError(t, os.Mkdir(target, 0o770))
	require.NoError(t, os.Chmod(target, 0o770))

	marker := filepath.Join(target, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("keep me"), 0o600))

	restoreUID := fakeCurrentUID(t, currentUID()+1)
	defer restoreUID()

	restoreMount := fakeMountPoint(t, target)
	defer restoreMount()

	require.NoError(t, MkdirAll(target, 0o700))

	content, err := os.ReadFile(marker)
	require.NoError(t, err, "a mount point owned by a group the user belongs to should be reused, not wiped")
	assert.Equal(t, "keep me", string(content))
}

// TestMkdirAll_UnrelatedGroupNonMountPointStillReplaced guards against over-widening the
// group exemption: a group/other-writable directory that is NOT a distinct mount point must
// still be treated as unsafe and wiped even when the current user's group matches its owning
// group, matching the same rule already enforced for uid ownership.
//
//nolint:paralleltest
func TestMkdirAll_UnrelatedGroupNonMountPointStillReplaced(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "same-device-group")
	require.NoError(t, os.Mkdir(target, 0o770))
	require.NoError(t, os.Chmod(target, 0o770))

	marker := filepath.Join(target, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("should be gone"), 0o600))

	restoreUID := fakeCurrentUID(t, currentUID()+1)
	defer restoreUID()

	require.NoError(t, MkdirAll(target, 0o700))

	_, err := os.Stat(marker)
	assert.True(t, os.IsNotExist(err), "group-writable non-mount-point directory should still be wiped")
}

// TestOwnedByUserOrGroup unit-tests the ownership check in isolation, covering the case
// isSafeExistingDir's integration tests above can't reach without root: a directory owned by
// neither the current user nor any of its groups must be rejected outright.
//
//nolint:paralleltest
func TestOwnedByUserOrGroup(t *testing.T) {
	scenarios := []struct {
		name     string
		dirUID   uint32
		dirGid   uint32
		fakeUID  int
		fakeGids []int
		want     bool
	}{
		{
			name:     "owned by current user",
			dirUID:   1000,
			dirGid:   2000,
			fakeUID:  1000,
			fakeGids: nil,
			want:     true,
		},
		{
			name:     "not owned by current user but group matches",
			dirUID:   0,
			dirGid:   2000,
			fakeUID:  1000,
			fakeGids: []int{2000},
			want:     true,
		},
		{
			name:     "neither user nor group matches",
			dirUID:   0,
			dirGid:   2000,
			fakeUID:  1000,
			fakeGids: []int{3000},
			want:     false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			restoreUID := fakeCurrentUID(t, scenario.fakeUID)
			defer restoreUID()

			restoreGroups := fakeCurrentGroups(t, scenario.fakeGids)
			defer restoreGroups()

			var stat syscall.Stat_t

			stat.Uid = scenario.dirUID
			stat.Gid = scenario.dirGid

			assert.Equal(t, scenario.want, ownedByUserOrGroup(&stat))
		})
	}
}

// fakeCurrentUID overrides currentUID so the current process appears to have uid, simulating
// a directory owned by a different user without requiring the test process to actually run as
// that user. It restores the original currentUID on cleanup.
func fakeCurrentUID(t *testing.T, uid int) func() {
	t.Helper()

	original := currentUID
	currentUID = func() int { return uid }

	return func() { currentUID = original }
}

// fakeCurrentGroups overrides currentGroups so the current process appears to belong to
// exactly gids, simulating a Kubernetes fsGroup supplementary group without requiring the
// test process to actually run under that group. It restores the original currentGroups on
// cleanup.
func fakeCurrentGroups(t *testing.T, gids []int) func() {
	t.Helper()

	original := currentGroups
	currentGroups = func() []int { return gids }

	return func() { currentGroups = original }
}

// fakeMountPoint overrides statDev so target appears to live on a different device than its
// parent, simulating a Kubernetes emptyDir/tmpfs mount without requiring an actual mount
// syscall (which needs root and is Linux-specific). It restores the original statDev on
// cleanup.
func fakeMountPoint(t *testing.T, target string) func() {
	t.Helper()

	parent := filepath.Dir(target)
	original := statDev

	statDev = func(path string) (uint64, bool) {
		if path == parent {
			return 999, true // any device number distinct from target's real one
		}

		return original(path)
	}

	return func() { statDev = original }
}
