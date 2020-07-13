package delta

import (
	"fmt"
	"io/ioutil"
	"os"
)

var EmptyID = ""

type LastEntityIDFileStore struct {
	readerFile func(path string) (string, error)
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

func (le *LastEntityIDFileStore) GetLastID() (string, error) {
	if !le.isEmpty() {
		return le.lastID, nil
	}

	v, err := le.readerFile(le.filePath)

	return v, err
}

func (le *LastEntityIDFileStore) isEmpty() bool {
	return le.lastID == EmptyID
}
