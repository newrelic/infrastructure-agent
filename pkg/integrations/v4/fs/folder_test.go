// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSFilesInFolderFn(t *testing.T) {
	dirPath, err := ioutil.TempDir("", "prefix")
	require.NoError(t, err)

	defer os.RemoveAll(dirPath)

	fileNameFoo := filepath.Join(dirPath, "foo")
	_, err = os.Create(fileNameFoo)
	require.NoError(t, err)
	fileNameBar := filepath.Join(dirPath, "bar")
	_, err = os.Create(fileNameBar)
	require.NoError(t, err)

	files, err := OSFilesInFolderFn(dirPath)
	require.NoError(t, err)
	assert.Equal(t, []string{fileNameBar, fileNameFoo}, files, "folder: "+dirPath)
}

func TestOSFilesInFolderFn_ErrNotFound(t *testing.T) {
	_, err := OSFilesInFolderFn("/path/to/nowhere")
	require.Equal(t, ErrFolderNotFound, err)
}
