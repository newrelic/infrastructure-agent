// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidYAMLs(t *testing.T) {
	t.Parallel()
	inputs := []struct {
		description string
		yaml        string
	}{{"simple aws-kms variable", `
variables:
  myData:
    aws-kms:
      data: T0hBSStGTEVY
      region: us-east-1
`}, {"simple vault variable", `
variables:
  myData:
    vault:
      http:
        url: http://www.example.com
`}, {"simple cyberark-cli variable", `
variables:
  myData:
    cyberark-cli:
      cli: /opt/local/bin/cberarkclie
      app-id: Postgres_Server
      safe: CYBER-ARK-SAFE
      folder: Root
      object: pg.newrelic.com
`}, {"simple cyberark-api variable", `
variables:
  myData:
    cyberark-api:
      http:
        url: https://10.1.0.5/AIMWebService/api/Accounts?AppID=NewRelic&Query=Safe=ALL-NERE-WIN-A-NEWRELIC-UP;Object=ALL-localhost-testuser
`}}
	for _, input := range inputs {
		t.Run(input.description, func(t *testing.T) {
			_, err := LoadYAML([]byte(input.yaml))
			assert.NoError(t, err)
		})
	}
}

func TestInvValidYAMLs(t *testing.T) {
	t.Parallel()
	inputs := []struct {
		description string
		yaml        string
	}{{"old format", `
variables:
  - aws-kms:
      data: T0hBSStGTEVY
      region: us-east-1
`}, {"no content", `
variables:
  myData:
`}, {"incomplete aws-kms variable", `
variables:
  myData:
    aws-kms:
      region: us-east-1
`}, {"empty variable name", `
variables:
  :    
    aws-kms:
      data: T0hBSStGTEVY
      region: us-east-1
`}, {"two discovery entries", `
variables:
  myData:    
    aws-kms:
      data: T0hBSStGTEVY
      region: us-east-1
    vault:
      http:
        url: http://www.example.com
`}, {"incomplete cyberark-cli variable", `
variables:
  myData:
    cyberark-cli:
      app-id: Postgres_Server
      safe: CYBER-ARK-SAFE
      folder: Root
      object:
      `}, {"incomplete cyberark-api variable", `
variables:
  myData:
    cyberark-api:
      http:
        url: 
      `}}
	for _, input := range inputs {
		t.Run(input.description, func(t *testing.T) {
			_, err := LoadYAML([]byte(input.yaml))
			assert.Error(t, err)
		})
	}
}
