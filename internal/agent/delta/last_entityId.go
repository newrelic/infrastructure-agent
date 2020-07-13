package delta

var EmptyId= ""

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
	if !le.isEmpty() {
		return le.lastID, nil
	}

	v, err := le.readerFile()
	if err != nil {
		return EmptyId, err
	}
	return v, err
}

func (le *LastEntityIDFileStore) isEmpty() bool {
	return le.lastID == EmptyId
}
