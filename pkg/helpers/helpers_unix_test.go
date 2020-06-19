// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestHostProc(t *testing.T) {
	path := HostProc("/test")
	assert.Equal(t, filepath.Join("/proc/test"), path)
	require.NoError(t, os.Setenv("HOST_PROC", "/dockerproc"))
	defer func() { require.NoError(t, os.Unsetenv("HOST_PROC")) }()
	newPath := HostProc("/testing")
	assert.Equal(t, filepath.Join("/dockerproc/testing"), newPath)
}

func TestHostVar(t *testing.T) {
	path := HostVar("/test/something/something")
	assert.Equal(t, filepath.Join("/var/test/something/something"), path)
	require.NoError(t, os.Setenv("HOST_VAR", "/dockervar"))
	defer func() { require.NoError(t, os.Unsetenv("HOST_PROC")) }()
	newPath := HostVar("/testing", "test")
	assert.Equal(t, filepath.Join("/dockervar/testing/test"), newPath)
}
