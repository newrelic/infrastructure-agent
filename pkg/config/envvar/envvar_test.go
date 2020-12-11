// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package envvar

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandInContent(t *testing.T) {
	emptyEnv := map[string]string{}

	tests := []struct {
		name    string
		env     map[string]string
		content string
		want    string
		wantErr bool
	}{
		{"empty", emptyEnv, "", "", false},
		{"no placeholder", emptyEnv, "foo bar\nbaz", "foo bar\nbaz", false},
		{"1 placeholder with no env-var", emptyEnv, "foo: {{BAR}}\nbaz", "", true},
		{"1 placeholder with 1 env-var", map[string]string{"BAR": "VAL"}, "foo: {{BAR}}\nbaz", "foo: VAL\nbaz", false},
		{"1 placeholder with 1 env-var with spaces", map[string]string{"BAR": "VAL"}, "foo: {{  BAR  }}\nbaz", "foo: VAL\nbaz", false},
		{"3 placeholder with 1 env-var", map[string]string{"BAR": "VAL"}, "foo: {{BAR}}\nbaz: {{BAR}}-{{BAR}}", "foo: VAL\nbaz: VAL-VAL", false},
		{"2 placeholder with 2 env-var", map[string]string{"BAR1": "VAL1", "BAR2": "VAL2"}, "foo: {{BAR1}}\nbaz: {{BAR2}}", "foo: VAL1\nbaz: VAL2", false},
		{"1 placeholder with 1 env-var special chars", map[string]string{"BAR": "$.*^"}, "foo: {{BAR}}\nbaz", "foo: $.*^\nbaz", false},
		{"1 placeholder with 1 env-var numeric", map[string]string{"BAR": "1"}, "foo: {{BAR}}", "foo: 1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				require.NoError(t, os.Setenv(k, v))
			}
			gotContent, gotErr := ExpandInContent([]byte(tt.content))
			if tt.wantErr {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
			assert.Equal(t, tt.want, string(gotContent))
		})
	}
}
