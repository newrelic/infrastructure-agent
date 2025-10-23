// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseWhoOutput(t *testing.T) {
	t.Parallel()

	var outputs = []string{
		`vagrant  pts/0        Oct 24 14:26 (10.0.2.2)
vagrant  pts/1        Oct 24 14:26 (10.0.2.2)
newrelic  pts/1        Oct 24 14:26 (10.0.2.2)`,
		`vagrant  pts/0        2018-10-24 15:55 (10.0.2.2)",
vagrant  pts/1        2018-10-24 15:55 (10.0.2.2)
newrelic  pts/1        2018-10-24 15:55 (10.0.2.2)`,
	}

	var expectedUsers = map[string]bool{
		"vagrant":  true,
		"newrelic": true,
	}
	for _, output := range outputs {
		users := parseWhoOutput(output)
		assert.Equal(t, expectedUsers, users)
	}
}
