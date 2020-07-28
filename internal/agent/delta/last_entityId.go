// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package delta

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type EntityIDPersist interface {
	GetEntityID() (entity.ID, error)
	UpdateEntityID(id entity.ID) error
}

// EntityIDFilePersist will store on the given file the EntityID in order to persist it between agent restarts.
type EntityIDFilePersist struct {
	readFile     func(path string) (entity.ID, error)
	writeFile    func(content entity.ID, path string) error
	filePath     string
	lastEntityID entity.ID
}

// NewEntityIDFilePersist create a new instance of EntityIDFilePersist.
func NewEntityIDFilePersist(dataDir string, fileName string) *EntityIDFilePersist {
	return &EntityIDFilePersist{
		readFile:  readFileFn,
		writeFile: writeFileFn,
		filePath:  filepath.Join(dataDir, lastEntityIDFolder, helpers.SanitizeFileName(fileName)),
	}
}

// GetEntityID will return entityID from memory or disk.
func (e *EntityIDFilePersist) GetEntityID() (entity.ID, error) {
	var err error
	if e.lastEntityID == entity.EmptyID {
		e.lastEntityID, err = e.readFile(e.filePath)
	}

	return e.lastEntityID, err
}

// UpdateEntityID will store the entityID on memory and disk.
func (e *EntityIDFilePersist) UpdateEntityID(id entity.ID) error {
	e.lastEntityID = id

	return e.writeFile(id, e.filePath)
}

func readFileFn(filePath string) (entity.ID, error) {
	// Check if there is an already stored value on disk.
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return entity.EmptyID, nil
	}

	buf, err := ioutil.ReadFile(filePath)

	if err != nil {
		return entity.EmptyID, fmt.Errorf("cannot read file persisted entityID, file: '%s', error: %v", filePath, err)
	}

	value, err := strconv.ParseInt(string(buf), 10, 64)
	if err != nil {
		return entity.EmptyID, fmt.Errorf("cannot parse entityID from file content: '%s', error: %v", buf, err)
	}

	return entity.ID(value), nil
}

func writeFileFn(content entity.ID, filePath string) error {
	dir := filepath.Dir(filePath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkDirErr := os.MkdirAll(dir, DATA_DIR_MODE); mkDirErr != nil {
			return fmt.Errorf("cannot persist entityID, agent data directory: '%s' does not exist and cannot be created: %v",
				dir, mkDirErr)
		}
	}

	return ioutil.WriteFile(filePath, []byte(content.String()), DATA_FILE_MODE)
}
