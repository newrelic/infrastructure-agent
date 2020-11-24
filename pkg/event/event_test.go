// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsReserved(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  bool
	}{
		{"empty", "", true},
		{"timestamp", "timestamp", true},
		{"hostname", "hostname", true},
		{"non reserved", "non-reserved", false},
		{"case independent", "entityID", true},
		{"attribute prefix", "attr.foo", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsReserved(tt.field))
		})
	}
}
