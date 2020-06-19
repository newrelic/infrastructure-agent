// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestFBSupervisorConfig_IsLogForwarderAvailable(t *testing.T) {
	// GIVEN
	file, err := ioutil.TempFile("", "nr_fb_config")
	if err != nil {
		assert.FailNow(t, "Could not create temporary testing file")
	}
	existing := file.Name()
	nonExisting := "non-existing-file"

	// GIVEN / THEN
	tests := []struct {
		name string
		cfg  FBSupervisorConfig
		want bool
	}{
		{
			"incorrect: all non-existing",
			FBSupervisorConfig{
				FluentBitExePath:     nonExisting,
				FluentBitNRLibPath:   nonExisting,
				FluentBitParsersPath: nonExisting,
			},
			false,
		},
		{
			"incorrect: NR lib and parsers do not exist",
			FBSupervisorConfig{
				FluentBitExePath:     existing,
				FluentBitNRLibPath:   nonExisting,
				FluentBitParsersPath: nonExisting,
			},
			false,
		},
		{
			"incorrect: parsers doesn't exist",
			FBSupervisorConfig{
				FluentBitExePath:     existing,
				FluentBitNRLibPath:   existing,
				FluentBitParsersPath: nonExisting,
			},
			false,
		},
		{
			"correct configuration",
			FBSupervisorConfig{
				FluentBitExePath:     existing,
				FluentBitNRLibPath:   existing,
				FluentBitParsersPath: existing,
			},
			true,
		},
	}

	// WHEN
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			available := tt.cfg.IsLogForwarderAvailable()

			assert.Equal(t, tt.want, available)
		})
	}

	// Teardown
	file.Close()
	if err := os.Remove(existing); err != nil {
		assert.FailNow(t, "Could not remove temporary test file")
	}
}
