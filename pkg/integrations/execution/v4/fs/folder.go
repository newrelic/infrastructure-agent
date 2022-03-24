// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fs

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

// FilesInFolderFn returns the files (paths) within a folder (path).
type FilesInFolderFn func(folderPath string) (filePaths []string, err error)

const (
	readableMask = 0444
)

// Errors
var (
	ErrFolderNotFound = errors.New("folder not found")
	ErrFilesNotFound  = errors.New("files not found")
)

var (
	// OSFilesInFolderFn returns the list of hopefully readable files for a folder.
	// This is a best effort guessing readability as official Go authors say:
	// > only reliable strategy is to just try to open it for read and see if it fails
	// https://golang-dev.narkive.com/FJOvNshy/isreadable-iswriteable-isfile-touch
	OSFilesInFolderFn = func(folderPath string) (filePaths []string, err error) {
		files, err := ioutil.ReadDir(folderPath)
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				err = ErrFolderNotFound
			}
			return
		}

		for _, file := range files {
			if !file.IsDir() {
				perm := file.Mode().Perm()
				if perm&readableMask == 0 {
					continue
				}
				filePaths = append(filePaths, filepath.Join(folderPath, file.Name()))
			}
		}

		if len(filePaths) == 0 {
			err = ErrFilesNotFound
		}

		return
	}
)
