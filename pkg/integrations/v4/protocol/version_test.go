// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolVersion_Undefined(t *testing.T) {
	p := PluginProtocolVersion{}
	for _, force := range []bool{false, true} {
		_, err := versionFromParsed(p, force)
		assert.Error(t, err)
	}
}

func TestProtocolVersion_InvalidProtocolType(t *testing.T) {
	for _, version := range []interface{}{4} {
		for _, force := range []bool{false, true} {
			p := PluginProtocolVersion{
				RawProtocolVersion: version,
			}
			_, err := versionFromParsed(p, force)
			assert.Error(t, err)
		}
	}
}

func TestProtocolVersion_IntegerTypes(t *testing.T) {
	dataCases := []struct {
		versions []interface{}
		name     string
		error    bool
	}{
		{
			versions: []interface{}{"3", "2", "1"},
			name:     "String integers are accepted",
			error:    false,
		},
		{
			versions: []interface{}{"foo", "3.32", "4.444", "3d"},
			name:     "Invalid strings",
			error:    true,
		},
		{
			versions: []interface{}{1.0, 2.0, 3.0},
			name:     "Float type: Major versions are accepted",
			error:    false,
		},
		{
			versions: []interface{}{1.3, 2.01, 3.30},
			name:     "Float type: Minor versions are not accepted.",
			error:    true,
		},
	}

	for _, dataCase := range dataCases {
		for _, v := range dataCase.versions {
			p := PluginProtocolVersion{
				RawProtocolVersion: v,
			}
			t.Run(dataCase.name, func(t *testing.T) {
				_, err := versionFromParsed(p, false)
				if dataCase.error {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	}
}

func TestProtocolVersion_InvalidProtocolNumber(t *testing.T) {
	for _, version := range []string{"5", "0"} {
		for _, force := range []bool{false, true} {
			p := PluginProtocolVersion{
				RawProtocolVersion: version,
			}
			_, err := versionFromParsed(p, force)
			assert.Error(t, err)
		}
	}
}

func TestProtocolVersion_ForceV3WhenIsV2(t *testing.T) {
	p := PluginProtocolVersion{
		RawProtocolVersion: "2",
	}
	v, err := versionFromParsed(p, true)
	assert.NoError(t, err)

	assert.Equal(t, 3, v)
}

func TestProtocolVersion_DoesNotForceV3WhenIsV1(t *testing.T) {
	p := PluginProtocolVersion{
		RawProtocolVersion: "1",
	}
	v, err := versionFromParsed(p, true)
	assert.NoError(t, err)

	assert.Equal(t, 1, v)
}
