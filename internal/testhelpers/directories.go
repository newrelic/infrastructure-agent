// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testhelpers

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

type mockDir struct {
	Path string
}

type MockFile struct {
	ParentDir string
	Name      string
	Content   string
}

func (dir mockDir) Clear() {
	os.RemoveAll(dir.Path)
}

// AddFile adds a file in the `mockDir` into the given path and with the
// given content.
func (dir mockDir) AddFile(file MockFile) error {
	absParentDir := filepath.Join(dir.Path, file.ParentDir)
	if file.ParentDir != "" {
		if err := os.MkdirAll(absParentDir, 0755); err != nil {
			return err
		}
	}
	filePath := filepath.Join(absParentDir, file.Name)
	if err := ioutil.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
		return err
	}
	return nil
}

// NewMockDir mocks a system directory and creates the given `MockFile`
// inside the new temp directory.
func NewMockDir(files []MockFile) (mockDir, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return mockDir{}, err
	}

	mDir := mockDir{
		Path: dir,
	}
	for _, mf := range files {
		if err = mDir.AddFile(mf); err != nil {
			return mDir, err
		}
	}

	return mDir, nil
}
