// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package files

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var allowedExtensions = map[string]struct{}{".yml": {}, ".yaml": {}}

// AllYAMLs returns FileInfo for all YAMLs in a folder
func AllYAMLs(folder string) ([]os.FileInfo, error) {
	fileInfos, err := ioutil.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var yamls []os.FileInfo
	for _, file := range fileInfos {
		if IsYAMLFile(file) {
			yamls = append(yamls, file)
		}
	}
	return yamls, nil
}

// IsYAMLFile returns it the passed object is a YAML and a file
func IsYAMLFile(file os.FileInfo) bool {
	if file.IsDir() {
		return false
	}
	_, ok := allowedExtensions[strings.ToLower(filepath.Ext(file.Name()))]
	return ok
}
