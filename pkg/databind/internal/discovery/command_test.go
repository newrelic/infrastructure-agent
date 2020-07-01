// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExecutable_Validate(t *testing.T) {
	type fields struct {
		Exec        ShlexOpt
		Environment map[string]string
		Matcher     map[string]string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr string
	}{
		{
			name: "Command missing",
			fields: fields{
				Matcher: map[string]string{
					"some": "matcher",
				},
			},
			wantErr: "missing 'cmd' entries",
		},
		{
			name: "Matcher missing",
			fields: fields{
				Exec: ShlexOpt{"/usr/bin/cmd"},
			},
			wantErr: "missing 'match' entries",
		},
		{
			name: "Happy",
			fields: fields{
				Exec: ShlexOpt{"/usr/bin/cmd"},
				Matcher: map[string]string{
					"some": "matcher",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Command{
				Exec:        tt.fields.Exec,
				Environment: tt.fields.Environment,
				Matcher:     tt.fields.Matcher,
			}
			err := e.Validate()
			if (err == nil) && (len(tt.wantErr) > 0) {
				assert.FailNow(t, "Wanted an error with message", tt.wantErr)
			}
			if (err != nil) && (len(tt.wantErr) < 1) {
				require.NoError(t, err)
			}
			if len(tt.wantErr) > 0 {
				require.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
