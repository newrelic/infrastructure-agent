package delta

import (
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


