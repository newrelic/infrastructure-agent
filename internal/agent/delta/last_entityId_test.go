package delta

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestLastEntityID_RetrieveStoredValue(t *testing.T) {
	expectedID := "entity_id"

	le := &LastEntityIDFileStore{
		readerFile: func(path string) (string, error) {
			return expectedID, nil
		},
	}

	id, err := le.GetLastID()

	assert.Equal(t, expectedID, id)
	assert.NoError(t, err)
}

func TestLastEntityID_RetrieveInMemoryValue(t *testing.T) {
	expectedID := "entity_id"

	le := &LastEntityIDFileStore{
		readerFile: func(path string) (string, error) {
			return EmptyID, fmt.Errorf("should not read from file")
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
		readerFile: func(path string) (string, error) {
			return EmptyID, fmt.Errorf(expectedMessage)
		},
	}

	id, err := le.GetLastID()

	assert.Equal(t, id, EmptyID)
	assert.Error(t, err, expectedMessage)
}

func TestLastEntityID_UpdateValue(t *testing.T) {
	expectedID := "new_id"

	le := &LastEntityIDFileStore{
		writerFile: func(id string) error {
			return nil
		},
	}

	err := le.UpdateLastID(expectedID)
	require.NoError(t, err)
	assert.Equal(t, expectedID, le.lastID)
}

func TestLastEntityID_ErrWhenWritingFile(t *testing.T) {
	expectedErrMessage := "file could not be written"
	expectedID := "expected_id"

	le := &LastEntityIDFileStore{
		writerFile: func(id string) error {
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
	expected := "some_content"

	path, err := TempDeltaStoreDir()
	assert.NoError(t, err)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(expected), 0644)
	assert.NoError(t, err)

	content, err := readFile(file)

	assert.Equal(t, expected, content)
	assert.NoError(t, err)
}

func TestReadFile_EmptyFile(t *testing.T) {
	expectedErrorMessage := "file has no content"

	path, err := TempDeltaStoreDir()
	assert.NoError(t, err)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(""), 0644)
	assert.NoError(t, err)

	content, err := readFile(file)

	assert.Equal(t, EmptyID, content)
	require.Error(t, err, "Expected to return an Empty file error")
	assert.Equal(t, expectedErrorMessage, err.Error())
}

func TestReadFile_FileNotFound(t *testing.T) {
	expectedErrorMessage := "stat some_non-existing_file_path: no such file or directory"

	content, err := readFile("some_non-existing_file_path")

	assert.Equal(t, EmptyID, content)
	require.Error(t, err, "Expected to failed when read a non-exiting file")
	assert.Equal(t, expectedErrorMessage, err.Error())
}
