package delta

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
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
func NewEntityIDFilePersist(dataDir string, entityKey string) *EntityIDFilePersist {
	return &EntityIDFilePersist{
		readFile:  readFileFn,
		writeFile: writeFileFn,
		filePath:  filepath.Join(dataDir, lastEntityIDFolder, entityKey),
	}
}

// GetEntityID will return entityID from memory or disk.
func (le *EntityIDFilePersist) GetEntityID() (entity.ID, error) {
	if le.lastEntityID != entity.EmptyID {
		return le.lastEntityID, nil
	}

	return le.readFile(le.filePath)
}

// UpdateEntityID will store the entityID on memory and disk.
func (le *EntityIDFilePersist) UpdateEntityID(id entity.ID) error {
	le.lastEntityID = id

	return le.writeFile(id, le.filePath)
}

func readFileFn(filePath string) (entity.ID, error) {
	_, err := os.Stat(filePath)

	// Check if there is an already stored value on disk.
	if os.IsNotExist(err) {
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

	entityID := entity.ID(value)

	return entityID, nil
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
