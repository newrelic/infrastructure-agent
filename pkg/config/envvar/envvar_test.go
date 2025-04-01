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
		// comments removal
		{"1 placeholder within comment lines are stripped", emptyEnv, "#foo: {{BAR}}\nbaz", "baz", false},
		{"comment lines starting with spaces are stripped", emptyEnv, "  #foo: {{BAR}}\nbaz", "baz", false},
		{"comment lines starting with tab are stripped", emptyEnv, "\t #foo: {{BAR}}\nbaz", "baz", false},
		{"comment part of the line is dropped while previous content is kept", emptyEnv, "foo: bar # {{BAR}}\nbaz", "foo: bar \nbaz", false},
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

func Test_removeYAMLComments(t *testing.T) {
	noComments := `integration_name: com.newrelic.mysql
	
	instances:
	- name: foo-bar
	`
	comments := `integration_name: com.newrelic.mysql

instances:
  - name: "foo '#name'"
    command: 'bar #Command' 
    arguments:
     r0: thisIsAValid#YAML
     r1: thisIsAValid	#comment
     r3: "f	#oo" # comment
     k0: foo # comment
     k1: foo # comment
     k2: "foo" # comment
     k3: "foo"# comment
     k4: "foo" # comment
     k5: "f#oo"# comment
     k6: "f#oo" # comment
     k7: "foo"# comment "foo"
     k8: "foo#bar"
     k9: ["foo#bar", "baz" ]  # another inline comment
     q1: 'foo' # comment
     q2: 'foo'# comment
     q3: 'foo' # comment
     q4: 'f#oo'# comment
     q5: 'f#oo' # comment
     q6: 'foo'# comment "foo"
     q7: 'foo#bar'
     q8: ['foo#bar', 'baz' ]  # another inline comment
	 q9: 'foo "bar" "#bar"'
	 q10: "foo \" bar"
     # some comments
     # some comments
    labels:
      foo: bar
`
	commentsStripped := `integration_name: com.newrelic.mysql

instances:
  - name: "foo '#name'"
    command: 'bar #Command' 
    arguments:
     r0: thisIsAValid#YAML
     r1: thisIsAValid	
     r3: "f	#oo" 
     k0: foo 
     k1: foo 
     k2: "foo" 
     k3: "foo"
     k4: "foo" 
     k5: "f#oo"
     k6: "f#oo" 
     k7: "foo"
     k8: "foo#bar"
     k9: ["foo#bar", "baz" ]  
     q1: 'foo' 
     q2: 'foo'
     q3: 'foo' 
     q4: 'f#oo'
     q5: 'f#oo' 
     q6: 'foo'
     q7: 'foo#bar'
     q8: ['foo#bar', 'baz' ]  
	 q9: 'foo "bar" "#bar"'
	 q10: "foo \" bar"
    labels:
      foo: bar
`

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{"empty", ``, ``, false},
		{"no comments", noComments, noComments, false},
		{"comments", comments, commentsStripped, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := removeYAMLComments([]byte(tt.content))
			assert.Equal(t, tt.want, string(got))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
