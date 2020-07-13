package delta

import (
	"fmt"
	"gotest.tools/assert"
	"testing"
)


func TestLastEntityId_RetrieveStoredValue(t *testing.T) {
	le := &LastEntityIDFileStore{
		readerFile: func() (string, error) {
			return "entity_id", nil
		},
	}

	id, _ := le.GetLastID()

	assert.Equal(t, id, "entity_id")
}

func TestLastEntityId_ErrWhenReadFile(t *testing.T) {
	le := &LastEntityIDFileStore{
		readerFile: func() (string, error) {
			return "", fmt.Errorf("failed when reading file")
		},
	}

	id, err := le.GetLastID()

	assert.Equal(t, id, EmptyId)
	assert.Error(t, err, "failed when reading file")
}



