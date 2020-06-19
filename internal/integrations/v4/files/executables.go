// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// package cmdlookup implements tools to recursively look for executables in a set of internal Folders
package files

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var clog = log.WithComponent("integrations.Executables")

// Executables stores the commands for each executable name, the corresponding complete path to the executable
type Executables struct {
	// Folders where the component have to look for executables with a given name
	Folders []string
}

// Path to an executable in the Folders set, given the executable name. The executable name
// is usually the file base name for the executable. In windows, is the file name without the ".exe" extension
func (fp Executables) Path(name string) (string, error) {
	cclog := clog.WithField("forName", name)
	for _, folder := range fp.Folders {
		fclog := cclog.WithField("folder", folder)
		fileInfos, err := ioutil.ReadDir(folder)
		if err != nil {
			fclog.WithError(err).
				Debug("Error looking for integration executables in folder. Trying another folder, if any.")
			continue
		}
		for _, file := range fileInfos {
			fileName := file.Name()
			if nameFor(fileName) == name {
				return filepath.Join(folder, fileName), nil
			}
			fclog.WithField("file", fileName).Debug("File does not match.")
		}
		fclog.Debug("Integration name not found. Trying another folder, if any.")
	}
	return "", errors.New("can't find an executable given the name: " + name)
}

// gets the proper name for a given file path
func nameFor(path string) string {
	base := filepath.Base(path)
	if runtime.GOOS != "windows" {
		return base
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}
