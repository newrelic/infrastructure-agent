// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fs

import (
	"errors"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/files"
	"os"
)

var (
	// ErrGetFileStats is returned when it fails to get file stat.
	ErrGetFileStats = errors.New("can't get stat for file")
	// ErrNotYAMLFile is returned in case the path doesn't have a valid yaml extension.
	ErrNotYAMLFile = errors.New("file is a directory or is not an accepted YAML extension")
)

// ValidateYAMLFile checks if the file exists(excepting if it has been
// marked for deletion) and if its it has a YAML extension.
func ValidateYAMLFile(filename string, beingDeleted bool) error {
	file, err := os.Stat(filename)
	if err != nil && (!os.IsNotExist(err) || !beingDeleted) {
		return ErrGetFileStats
	}
	if file != nil && !files.IsYAMLFile(file) {
		return ErrNotYAMLFile
	}
	return nil
}
