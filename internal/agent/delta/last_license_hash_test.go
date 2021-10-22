package delta

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLicenseHashFilePersist_RetrieveStoredValue(t *testing.T) {
	expected := "718779752b851ac0dc6281a8c8d77e7e"

	l := &LastSubmissionLicenseFileStore{
		readFile: func(path string) (string, error) {
			return expected, nil
		},
	}

	err := l.load()

	assert.Equal(t, expected, l.licenseMD5)
	assert.NoError(t, err)
}

func TestLicenseHashFilePersist_RetrieveInMemoryValue(t *testing.T) {
	expected := "718779752b851ac0dc6281a8c8d77e7e"

	l := &LastSubmissionLicenseFileStore{
		readFile: func(path string) (string, error) {
			return "", fmt.Errorf("should not read from file")
		},
		licenseMD5: expected,
	}

	err := l.load()

	assert.Equal(t, expected, l.licenseMD5)
	require.NoError(t, err)
}

func TestLicenseHashFilePersist_ErrWhenReadingFile(t *testing.T) {
	expectedMessage := "failed when reading file"

	l := &LastSubmissionLicenseFileStore{
		readFile: func(path string) (string, error) {
			return "", fmt.Errorf(expectedMessage)
		},
	}

	err := l.load()

	assert.Equal(t, "", l.licenseMD5)
	assert.Error(t, err, expectedMessage)
}

func TestLicenseHashFilePersist_UpdateValue(t *testing.T) {
	license := "license"
	expected := "718779752b851ac0dc6281a8c8d77e7e"

	l := &LastSubmissionLicenseFileStore{
		writeFile: func(content string, filePath string) error {
			assert.Equal(t, expected, content)
			return nil
		},
	}

	err := l.Update(license)
	require.NoError(t, err)
	assert.Equal(t, expected, l.licenseMD5)
}

func TestLicenseHashFilePersist_ErrWhenWritingFile(t *testing.T) {
	expectedErrMessage := "file could not be written"
	expected := "718779752b851ac0dc6281a8c8d77e7e"

	l := &LastSubmissionLicenseFileStore{
		writeFile: func(content string, filePath string) error {
			return fmt.Errorf(expectedErrMessage)
		},
	}

	err := l.Update("license")

	assert.Errorf(t, err, "Update license should return an error when failed writing file")
	assert.Equal(t, expectedErrMessage, err.Error())
	assert.Equal(t, expected, l.licenseMD5)
}

// Read File IT
func TestReadLicenseHashFromFileFn_ReturnContent(t *testing.T) {
	expected := "718779752b851ac0dc6281a8c8d77e7e"

	path, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(path)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(expected), 0644)
	assert.NoError(t, err)

	content, err := readLicenseHashFromFileFn(file)

	assert.Equal(t, expected, content)
	assert.NoError(t, err)
}

func TestReadLicenseHashFromFileFn_EmptyFile(t *testing.T) {
	path, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(path)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(""), 0644)
	assert.NoError(t, err)

	content, err := readLicenseHashFromFileFn(file)

	assert.Equal(t, "", content)
	require.Error(t, err, "Expected to return an Empty file error")
}

func TestReadLicenseHashFromFileFn_FileNotFound(t *testing.T) {
	content, err := readLicenseHashFromFileFn("some_non-existing_file_path")

	assert.Equal(t, "", content)
	require.NoError(t, err)
}

func TestReadLicenseHashFromFileFn_NoPermission(t *testing.T) {
	path, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(path)

	file := filepath.Join(path, "temporary_file")
	err = ioutil.WriteFile(file, []byte(""), 0000)
	assert.NoError(t, err)

	content, err := readLicenseHashFromFileFn(file)

	assert.Equal(t, "", content)
	require.Error(t, err, "Expected to return an permission error")
}

func TestReadLicenseHashFromFileFn_ErrParseContent(t *testing.T) {
	temp, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(temp)

	filePath := filepath.Join(temp, "temporary_file")
	err = ioutil.WriteFile(filePath, []byte("wrong_md5"), 0644)
	require.NoError(t, err)

	emptyID, err := readLicenseHashFromFileFn(filePath)
	require.Error(t, err)
	assert.Equal(t, "", emptyID)
}

// Write File IT
func TestWriteLicenseHashToFileFn_StoreValue(t *testing.T) {

	//GIVEN a file with a stored license hash
	oldHash := "1bad9a4c31a1256e9d62feea2dcfd188"

	temp, err := TempDeltaStoreDir()
	require.NoError(t, err)
	defer cleanFolder(temp)

	err = os.MkdirAll(filepath.Join(temp, lastLicenseHashFolder), DATA_DIR_MODE)
	require.NoError(t, err)

	filePath := filepath.Join(temp, lastLicenseHashFolder, "nria_locally")
	err = ioutil.WriteFile(filePath, []byte(oldHash), DATA_FILE_MODE)
	require.NoError(t, err, "Should create a last license hash file")

	newLicense := "license"
	newHash := "718779752b851ac0dc6281a8c8d77e7e"
	l := NewLastSubmissionLicenseFileStore(temp, "nria_locally")

	//WHEN UpdateEntityID
	err = l.Update(newLicense)
	require.NoError(t, err)

	//THEN new content can be retrieved
	persistedLicesenHash, err := ioutil.ReadFile(filePath)
	require.NoError(t, err)

	assert.Equal(t, newHash, string(persistedLicesenHash))
}

func TestWriteLicenseHashToFileFn_FileNotExist(t *testing.T) {
	expected := "718779752b851ac0dc6281a8c8d77e7e"

	//GIVEN a temporary folder
	temp, err := TempDeltaStoreDir()
	require.NoError(t, err, "Should create a temporary file")
	defer cleanFolder(temp)

	//AND a non existing file to store a value
	nonExistingFilePath := filepath.Join(temp, "nria_locally")

	//WHEN want to write content on it
	err = writeLicenseHashToFileFn(expected, nonExistingFilePath)
	require.NoError(t, err, "Should write the file")

	//THEN file were created
	_, err = os.Stat(nonExistingFilePath)
	require.NoError(t, err, "Should create the file if not exist")

	//AND the value can be retrieved
	actualValue, err := readLicenseHashFromFileFn(nonExistingFilePath)
	require.NoError(t, err, "Should retrieve value from file")
	assert.Equal(t, expected, actualValue)
}

func TestHasChanged(t *testing.T) {
	oldLicense := "license"
	oldHash := "718779752b851ac0dc6281a8c8d77e7e"

	l := &LastSubmissionLicenseFileStore{
		readFile: func(path string) (string, error) {
			return "", fmt.Errorf("this should not be called")
		},
		licenseMD5: oldHash,
	}

	actual, actualErr := l.HasChanged(oldLicense)
	assert.False(t, actual)
	assert.NoError(t, actualErr)

	actual, actualErr = l.HasChanged("license2")
	assert.True(t, actual)
	assert.NoError(t, actualErr)
}
