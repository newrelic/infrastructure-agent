// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package acquire

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadLines(t *testing.T) {
	var readLinesTest = []struct {
		name          string
		lines         string
		expectedLines []string
	}{
		{"File ending with new line", "Hello\nGoodbye\n", []string{"Hello", "Goodbye"}},
		{"File ending without new line", "Hello\nGoodbye", []string{"Hello", "Goodbye"}},
		{"File with one line and ending without new line", "Goodbye", []string{"Goodbye"}},
		{"File with one line and ending with new line", "Goodbye\n", []string{"Goodbye"}},
		{"Empty file", "", nil},
	}
	for _, tt := range readLinesTest {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ioutil.TempFile("", "readlines")
			assert.NoError(t, err)
			defer os.Remove(f.Name())
			_, err = f.Write([]byte(tt.lines))
			assert.NoError(t, err)
			testLines, err := ReadLines(f.Name())
			assert.Equal(t, err, io.EOF)
			assert.Equal(t, tt.expectedLines, testLines)
		})
	}
}
