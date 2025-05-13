// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package delta

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityIDFilePersist_RetrieveStoredValue(t *testing.T) {
	expectedID := entity.ID(10)

	le := &EntityIDFilePersist{
		readFile: func(path string) (entity.ID, error) {
			return expectedID, nil
		},
	}

	id, err := le.GetEntityID()

	assert.Equal(t, expectedID, id)
	assert.NoError(t, err)
}

func TestEntityIDFilePersist_RetrieveInMemoryValue(t *testing.T) {
	expectedID := entity.ID(10)

	le := &EntityIDFilePersist{
		readFile: func(path string) (entity.ID, error) {
			return entity.EmptyID, fmt.Errorf("should not read from file")
		},
		lastEntityID: expectedID,
	}

	id, err := le.GetEntityID()

	assert.Equal(t, expectedID, id)
	require.NoError(t, err)
}

func TestEntityIDFilePersist_ErrWhenReadingFile(t *testing.T) {
	expectedMessage := "failed when reading file"

	le := &EntityIDFilePersist{
		readFile: func(path string) (entity.ID, error) {
			return entity.EmptyID, fmt.Errorf("%v", expectedMessage)
		},
	}

	id, err := le.GetEntityID()

	assert.Equal(t, id, entity.EmptyID)
	assert.Error(t, err, expectedMessage)
}

func TestEntityIDFilePersist_UpdateValue(t *testing.T) {
	expectedID := entity.ID(10)

	le := &EntityIDFilePersist{
		writeFile: func(id entity.ID, filePath string) error {
			assert.Equal(t, expectedID, id)
			return nil
		},
	}

	err := le.UpdateEntityID(expectedID)
	require.NoError(t, err)
	assert.Equal(t, expectedID, le.lastEntityID)
}

func TestEntityIDFilePersist_ErrWhenWritingFile(t *testing.T) {
	expectedErrMessage := "file could not be written"
	expectedID := entity.ID(10)

	le := &EntityIDFilePersist{
		writeFile: func(id entity.ID, filePath string) error {
			return fmt.Errorf("%v", expectedErrMessage)
		},
	}

	err := le.UpdateEntityID(expectedID)

	assert.Errorf(t, err, "Update lastEntityID should return an error when failed writing file")
	assert.Equal(t, expectedErrMessage, err.Error())
	assert.Equal(t, expectedID, le.lastEntityID)
}

// Read File IT
func TestReadFile_ReturnContent(t *testing.T) {
	expected := entity.ID(10)

	path, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(path)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(expected.String()), 0644)
	assert.NoError(t, err)

	content, err := readFileFn(file)

	assert.Equal(t, expected, content)
	assert.NoError(t, err)
}

func TestReadFile_EmptyFile(t *testing.T) {
	path, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(path)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(""), 0644)
	assert.NoError(t, err)

	content, err := readFileFn(file)

	assert.Equal(t, entity.EmptyID, content)
	require.Error(t, err, "Expected to return an Empty file error")
}

func TestReadFile_FileNotFound(t *testing.T) {
	content, err := readFileFn("some_non-existing_file_path")

	assert.Equal(t, entity.EmptyID, content)
	require.NoError(t, err)
}

func TestReadFile_NoPermission(t *testing.T) {
	path, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(path)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(""), 0000)
	assert.NoError(t, err)

	content, err := readFileFn(file)

	assert.Equal(t, entity.EmptyID, content)
	require.Error(t, err, "Expected to return an permission error")
}

func TestReadFile_ErrParseContent(t *testing.T) {
	temp, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(temp)

	filePath := filepath.Join(temp, "temporary_file")
	err = ioutil.WriteFile(filePath, []byte("wrong_entity_id"), 0644)
	require.NoError(t, err)

	emptyID, err := readFileFn(filePath)
	require.Error(t, err)
	assert.Equal(t, entity.EmptyID, emptyID)
}

// Write File IT
func TestWriteFile_StoreValue(t *testing.T) {

	//GIVEN a file with a stored entityID
	oldID := entity.ID(123456)

	temp, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(temp)

	err = os.MkdirAll(filepath.Join(temp, lastEntityIDFolder), DATA_DIR_MODE)
	require.NoError(t, err)

	filePath := filepath.Join(temp, lastEntityIDFolder, "serverentity_key0180")
	err = ioutil.WriteFile(filePath, []byte(oldID.String()), DATA_FILE_MODE)
	require.NoError(t, err, "Should create a last entity ID file")

	newID := entity.ID(54321)
	le := NewEntityIDFilePersist(temp, "server:entity_key01:80")

	//WHEN UpdateEntityID
	err = le.UpdateEntityID(newID)
	require.NoError(t, err)

	//THEN new content can be retrieved
	persistedID, err := ioutil.ReadFile(filePath)
	require.NoError(t, err)

	assert.Equal(t, newID.String(), string(persistedID))
}

func TestWriteFile_FileNotExist(t *testing.T) {
	expectedValue := entity.ID(10)

	//GIVEN a temporary folder
	temp, err := TempDeltaStoreDir()
	require.NoError(t, err, "Should create a temporary file")
	defer cleanFolder(temp)

	//AND a non existing file to store a value
	nonExistingFilePath := filepath.Join(temp, "last_entity_ID")

	//WHEN want to write content on it
	err = writeFileFn(expectedValue, nonExistingFilePath)
	require.NoError(t, err, "Should write the file")

	//THEN file were created
	_, err = os.Stat(nonExistingFilePath)
	require.NoError(t, err, "Should create the file if not exist")

	//AND the value can be retrieved
	actualValue, err := readFileFn(nonExistingFilePath)
	require.NoError(t, err, "Should retrieve value from file")
	assert.Equal(t, expectedValue, actualValue)
}

func cleanFolder(path string) {
	func() {
		_ = os.RemoveAll(path)
	}()
}
