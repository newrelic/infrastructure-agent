package delta

type LastEntityIDFileStore struct {
	readerFile func() (string, error)
	path       string
	lastID     string
}

func NewLastEntityId() *LastEntityIDFileStore {
	return &LastEntityIDFileStore{
		readerFile: readFile,
	}
}

func readFile() (string, error) {
	return "", nil
}

func (le *LastEntityIDFileStore) GetLastID() (string, error) {
	if le.lastID != "" {
		return le.lastID, nil
	}

	v, _ := le.readerFile()
	return v, nil
}
