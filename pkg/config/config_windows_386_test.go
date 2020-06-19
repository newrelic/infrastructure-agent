// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows
// +build 386

package config

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLegacyStorageSamplerWin32(t *testing.T) {
	// Test that missing fields are replaced by its default
	configStr := `
license_key: abc123
`
	f, err := ioutil.TempFile("", "opsmatic_config_test")
	assert.NoError(t, err)

	n, err := f.WriteString(configStr)
	assert.NoError(t, err)
	assert.EqualValues(t, n, len(configStr))

	err = f.Close()
	assert.NoError(t, err)

	cfg, err := LoadConfig(f.Name())
	assert.NoError(t, err)
	assert.True(t, cfg.LegacyStorageSampler)
}
