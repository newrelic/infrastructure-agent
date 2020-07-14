package delta

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type LastEntityIDFileStore struct {
	readerFile func(path string) (entity.ID, error)
	writerFile func(content entity.ID, path string) error
	filePath   string
	lastID     entity.ID
}

func NewLastEntityId() *LastEntityIDFileStore {
	return &LastEntityIDFileStore{
		readerFile: readFile,
	}
}

func readFile(filePath string) (entity.ID, error) {
	_, err := os.Stat(filePath)

	if os.IsNotExist(err) {
		return entity.EmptyID, err
	}

	buf, err := ioutil.ReadFile(filePath)

	if err != nil {
		return entity.EmptyID, err
	}

	s, _ := strconv.ParseInt(string(buf), 10, 64)

	e := entity.ID(s)

	if e == entity.EmptyID {
		return entity.EmptyID, fmt.Errorf("file has no content")
	}

	return e, nil
}

func writeFile(content entity.ID, filePath string) error {
	dir := filepath.Dir(filePath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, DATA_DIR_MODE)
	}

	return ioutil.WriteFile(filePath, []byte(content.String()), DATA_FILE_MODE)
}

func (le *LastEntityIDFileStore) GetLastID() (entity.ID, error) {
	if !le.isEmpty() {
		return le.lastID, nil
	}

	v, err := le.readerFile(le.filePath)

	return v, err
}

func (le *LastEntityIDFileStore) UpdateLastID(id entity.ID) error {
	le.lastID = id

	err := le.writerFile(id, le.filePath)

	if err != nil {
		return err
	}

	return nil
}

func (le *LastEntityIDFileStore) isEmpty() bool {
	return le.lastID == entity.EmptyID
}
