package delta

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLastEntityID_RetrieveStoredValue(t *testing.T) {
	expectedID := entity.ID(10)

	le := &LastEntityIDFileStore{
		readerFile: func(path string) (entity.ID, error) {
			return expectedID, nil
		},
	}

	id, err := le.GetLastID()

	assert.Equal(t, expectedID, id)
	assert.NoError(t, err)
}

func TestLastEntityID_RetrieveInMemoryValue(t *testing.T) {
	expectedID := entity.ID(10)

	le := &LastEntityIDFileStore{
		readerFile: func(path string) (entity.ID, error) {
			return entity.EmptyID, fmt.Errorf("should not read from file")
		},
		lastID: expectedID,
	}

	id, err := le.GetLastID()

	assert.Equal(t, expectedID, id)
	require.NoError(t, err)
}

func TestLastEntityID_ErrWhenReadingFile(t *testing.T) {
	expectedMessage := "failed when reading file"

	le := &LastEntityIDFileStore{
		readerFile: func(path string) (entity.ID, error) {
			return entity.EmptyID, fmt.Errorf(expectedMessage)
		},
	}

	id, err := le.GetLastID()

	assert.Equal(t, id, entity.EmptyID)
	assert.Error(t, err, expectedMessage)
}

func TestLastEntityID_UpdateValue(t *testing.T) {
	expectedID := entity.ID(10)

	le := &LastEntityIDFileStore{
		writerFile: func(id entity.ID, filePath string) error {
			return nil
		},
	}

	err := le.UpdateLastID(expectedID)
	require.NoError(t, err)
	assert.Equal(t, expectedID, le.lastID)
}

func TestLastEntityID_ErrWhenWritingFile(t *testing.T) {
	expectedErrMessage := "file could not be written"
	expectedID := entity.ID(10)

	le := &LastEntityIDFileStore{
		writerFile: func(id entity.ID, filePath string) error {
			return fmt.Errorf(expectedErrMessage)
		},
	}

	err := le.UpdateLastID(expectedID)

	assert.Errorf(t, err, "Update lastID should return an error when failed writing file")
	assert.Equal(t, expectedErrMessage, err.Error())
	assert.Equal(t, expectedID, le.lastID)
}

// Read File IT
func TestReadFile_ReturnContent(t *testing.T) {
	expected := entity.ID(10)

	path, err := TempDeltaStoreDir()
	assert.NoError(t, err)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(expected.String()), 0644)
	assert.NoError(t, err)

	content, err := readFile(file)

	assert.Equal(t, expected, content)
	assert.NoError(t, err)
}

func TestReadFile_EmptyFile(t *testing.T) {
	path, err := TempDeltaStoreDir()
	assert.NoError(t, err)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(""), 0644)
	assert.NoError(t, err)

	content, err := readFile(file)

	assert.Equal(t, entity.EmptyID, content)
	require.Error(t, err, "Expected to return an Empty file error")
}

func TestReadFile_FileNotFound(t *testing.T) {
	content, err := readFile("some_non-existing_file_path")

	assert.Equal(t, entity.EmptyID, content)
	require.Error(t, err, "Expected to failed when read a non-exiting file")
}

func TestReadFile_NoPermission(t *testing.T) {
	path, err := TempDeltaStoreDir()
	assert.NoError(t, err)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(""), 0000)
	assert.NoError(t, err)

	content, err := readFile(file)

	assert.Equal(t, entity.EmptyID, content)
	require.Error(t, err, "Expected to return an permission error")
}

func TestReadFile_ErrParseContent(t *testing.T) {

}

// Write File IT
func TestWriteFile_StoreValue(t *testing.T) {
	expectedValue := entity.ID(10)

	//GIVEN an empty file
	temp, err := TempDeltaStoreDir()
	filePath := filepath.Join(temp, "last_entity_ID")
	err = ioutil.WriteFile(filePath, []byte(entity.EmptyID.String()), DATA_FILE_MODE)
	require.NoError(t, err, "Should create a last entity ID file")

	//WHEN write new content on it
	err = writeFile(expectedValue, filePath)
	require.NoError(t, err, "Should create the file if not exist")

	//THEN new content can be retrieved
	actualValue, err := readFile(filePath)
	require.NoError(t, err, "Should retrieve value from file")
	assert.Equal(t, expectedValue, actualValue)
}

func TestWriteFile_FileNotExist(t *testing.T) {
	expectedValue := entity.ID(10)

	//GIVEN a temporary folder
	temp, err := TempDeltaStoreDir()
	require.NoError(t, err, "Should create a temporary file")

	//AND a non existing file to store a value
	nonExistingFilePath := filepath.Join(temp, "last_entity_ID")

	//WHEN want to write content on it
	err = writeFile(expectedValue, nonExistingFilePath)
	require.NoError(t, err, "Should write the file")

	//THEN file were created
	_, err = os.Stat(nonExistingFilePath)
	require.NoError(t, err, "Should create the file if not exist")

	//AND the value can be retrieved
	actualValue, err := readFile(nonExistingFilePath)
	require.NoError(t, err, "Should retrieve value from file")
	assert.Equal(t, expectedValue, actualValue)
}

//TODO write/override multiple times

