// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"testing"

	"gopkg.in/check.v1"
)

// Register test suite.
func TestSshdConfig(t *testing.T) {
	t.Parallel()
}

type SshdConfigSuite struct{}

var _ = check.Suite(&SshdConfigSuite{})

func (s *SshdConfigSuite) TestParseSshdConfig(c *check.C) {
	testInputs := map[string]struct {
		configText     string
		expectedConfig map[string]string
	}{
		"all_on": {
			configText: `
PermitRootLogin without-password
# a comment
SomeCoolSettings     no
PermitEmptyPasswords yes # a comment afterwards
 PasswordAuthentication      yes
Derp no
ChallengeResponseAuthentication yes
			`,
			expectedConfig: map[string]string{
				"PermitRootLogin":                 "without-password",
				"PermitEmptyPasswords":            "yes",
				"PasswordAuthentication":          "yes",
				"ChallengeResponseAuthentication": "yes",
			},
		},
		"tricky_comment": {
			configText: `
PermitRootLogin no
# PermitRootLogin yes
`,
			expectedConfig: map[string]string{
				"PermitRootLogin": "no",
			},
		},
	}

	for _, inputs := range testInputs {
		config, err := parseSshdConfig(inputs.configText)
		c.Check(err, check.IsNil)
		c.Check(config, check.DeepEquals, inputs.expectedConfig)
	}
}
