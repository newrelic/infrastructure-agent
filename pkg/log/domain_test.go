// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// agent domain features
package log

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithIntegration(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)

	WithIntegration("ethics").Warn("animals suffer as we do")

	written := output.String()
	assert.Contains(t, written, "animals suffer as we do")
	assert.Contains(t, written, "integration")
	assert.Contains(t, written, "ethics")
}

func TestEntry_WithIntegration_WithPlugin(t *testing.T) {
	var output bytes.Buffer
	SetOutput(&output)

	WithIntegration("foo").WithPlugin("bar").Warn("some msg")

	written := output.String()
	assert.Contains(t, written, "integration")
	assert.Contains(t, written, "foo")
	assert.Contains(t, written, "plugin")
	assert.Contains(t, written, "bar")
}
