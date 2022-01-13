// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package distro

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseDistro_Success(t *testing.T) {
	actual := parseDistro(func() (string, error) {
		return `ProductName:	macOS2
ProductVersion:	11.2.3
BuildVersion:	20D91`, nil
	})

	assert.Equal(t, "macOS2 11.2.3", actual)
}

func TestParseDistro_MissingProductName(t *testing.T) {
	actual := parseDistro(func() (string, error) {
		return `ProductVersion:	11.2.3
BuildVersion:	20D91`, nil
	})

	assert.Equal(t, "macOS 11.2.3", actual)
}

func TestParseDistro_MissingProductVersion(t *testing.T) {
	actual := parseDistro(func() (string, error) {
		return `BuildVersion:	20D91`, nil
	})

	assert.Equal(t, "macOS (Unknown)", actual)
}

func TestParseDistro_OnError(t *testing.T) {
	actual := parseDistro(func() (string, error) {
		return ``, fmt.Errorf("error")
	})

	assert.Equal(t, defaultProductName, actual)
}
