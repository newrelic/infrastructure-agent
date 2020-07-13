package delta

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

var EmptyID = ""

type LastEntityIDFileStore struct {
	readerFile func(path string) (string, error)
	writerFile func(content string, path string) error
	filePath   string
	lastID     string
}

func NewLastEntityId() *LastEntityIDFileStore {
	return &LastEntityIDFileStore{
		readerFile: readFile,
	}
}

func readFile(filePath string) (string, error) {
	_, err := os.Stat(filePath)

	if err != nil {
		return EmptyID, err
	}

	buf, err := ioutil.ReadFile(filePath)

	if err != nil {
		return EmptyID, err
	}

	s := string(buf)

	if s == EmptyID {
		return EmptyID, fmt.Errorf("file has no content")
	}

	return string(buf), nil
}

func writeFile(content string, filePath string) error {
	dir := filepath.Dir(filePath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, DATA_DIR_MODE)
	}

	return ioutil.WriteFile(filePath, []byte(content), DATA_DIR_MODE)
}

func (le *LastEntityIDFileStore) GetLastID() (string, error) {
	if !le.isEmpty() {
		return le.lastID, nil
	}

	v, err := le.readerFile(le.filePath)

	return v, err
}

func (le *LastEntityIDFileStore) UpdateLastID(id string) error {
	le.lastID = id

	err := le.writerFile(id, le.filePath)

	if err != nil {
		return err
	}

	return nil
}

func (le *LastEntityIDFileStore) isEmpty() bool {
	return le.lastID == EmptyID
}
