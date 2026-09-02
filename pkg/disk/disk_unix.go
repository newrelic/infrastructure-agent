// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

// package disk provides access to common disk write operations
package disk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// WriteFile is a façade to ioutil.WriteFile, which enforces safe disk access when the host configuration requires it
var WriteFile = os.WriteFile //nolint:gochecknoglobals

// OpenFile is a façade to os.OpenFile, which enforces safe disk access when the host configuration requires it
var OpenFile = os.OpenFile

// Create is a façade to os.Create, which enforces safe disk access when the host configuration requires it
var Create = os.Create

// ErrUnsafeDirAfterCreate is returned by MkdirAll when the target path is not safe to use
// right after being (re)created, most likely because another local user won the narrow
// remove-then-create race window.
var ErrUnsafeDirAfterCreate = errors.New("path is not safe to use after creation, possibly due to a race")

// MkdirAll creates a directory path along with any necessary parents, like os.MkdirAll.
// Unlike os.MkdirAll, it never blindly reuses a pre-existing path: if path already exists
// but is not a real directory owned by the current user, or is writable by group/other, it
// is removed and recreated fresh. This prevents a local attacker from planting (or
// symlinking) a predictable path - e.g. under a world-writable /tmp - that a privileged
// process would otherwise pick up and reuse without noticing.
func MkdirAll(path string, perm os.FileMode) error {
	pathInfo, err := os.Lstat(path)

	switch {
	case os.IsNotExist(err):
		// Nothing to reuse, fall through to create below.
	case err != nil:
		return fmt.Errorf("failed to stat %q: %w", path, err)
	case isSafeExistingDir(path, pathInfo):
		return nil
	default:
		rmErr := os.RemoveAll(path)
		if rmErr != nil {
			return fmt.Errorf("refusing to reuse unsafe path %q, and failed to remove it: %w", path, rmErr)
		}
	}

	err = os.MkdirAll(path, perm)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", path, err)
	}

	// Re-check after creation to close the remove-then-create race: if another local
	// user won that narrow window, fail loudly instead of silently using a directory we
	// don't actually own.
	pathInfo, err = os.Lstat(path)
	if err != nil {
		return fmt.Errorf("failed to stat %q after creation: %w", path, err)
	}

	if !isSafeExistingDir(path, pathInfo) {
		return fmt.Errorf("%q: %w", path, ErrUnsafeDirAfterCreate)
	}

	return nil
}

// isSafeExistingDir reports whether pathInfo describes a real directory (not a symlink)
// owned by the current user, that is either not writable by group/other, or is a distinct
// mount point (e.g. a Kubernetes emptyDir/tmpfs volume).
//
// A separate mount point is set up by a privileged process (the container runtime/kubelet)
// before the agent ever runs, not planted by an arbitrary local user - the "predictable
// writable path" attack MkdirAll guards against requires being able to create the path
// ahead of time under a shared, writable parent directory (e.g. /tmp), which isn't possible
// for a distinct filesystem mount. Kubernetes commonly mounts emptyDir volumes as
// world/group-writable (mode 0777), so without this exemption every such mount is wrongly
// treated as unsafe and removed - which then fails outright when the mount is the root of a
// read-only-root-filesystem container, since the mount point itself can't be unlinked from
// its (read-only) parent.
func isSafeExistingDir(path string, pathInfo os.FileInfo) bool {
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.IsDir() {
		return false
	}

	stat, ok := pathInfo.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return false
	}

	//nolint:unconvert // stat.Dev is already uint64 on this arch (redundant here), but int32 on
	// others (e.g. darwin/amd64) where the widening conversion is required; kept explicit for portability
	dev := uint64(stat.Dev)

	if pathInfo.Mode().Perm()&0o022 != 0 && !isMountPoint(path, dev) {
		return false
	}

	return true
}

// statDevImpl returns the device number of the filesystem containing path.
func statDevImpl(path string) (uint64, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, false
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}

	//nolint:unconvert // stat.Dev is already uint64 on this arch (redundant here), but int32 on
	// others (e.g. darwin/amd64) where the widening conversion is required; kept explicit for portability
	return uint64(stat.Dev), true
}

// statDev is a package-level var, like the WriteFile/OpenFile/Create façades above, so tests
// can simulate a distinct mount without needing an actual mount syscall.
var statDev = statDevImpl //nolint:gochecknoglobals

// isMountPoint reports whether path is the root of a distinct filesystem mount, i.e. its
// device number differs from its parent directory's. A failure to resolve the parent's
// device is treated as "not a mount point" so callers fall back to the stricter permission
// check.
func isMountPoint(path string, dev uint64) bool {
	parentDev, ok := statDev(filepath.Dir(path))

	return ok && parentDev != dev
}
