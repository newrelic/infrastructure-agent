package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateConfigKey(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "newrelic-infra.yml")

	initialYAML := []byte("# This is a crucial user comment\ndisplay_name: old-server\n")
	err := os.WriteFile(filePath, initialYAML, 0644)
	require.NoError(t, err, "Should create temp file without error")

	err = UpdateConfigKey(filePath, "display_name", "new-production-server")
	require.NoError(t, err, "Should update config without error")

	updatedYAML, err := os.ReadFile(filePath)
	require.NoError(t, err)

	assert.Contains(t, string(updatedYAML), "display_name: new-production-server")
	assert.Contains(t, string(updatedYAML), "# This is a crucial user comment", "Comments MUST be preserved")

	err = UpdateConfigKey(filePath, "custom_attributes", "{env: prod}")
	require.NoError(t, err)

	finalYAML, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(finalYAML), "custom_attributes: '{env: prod}'")
}
