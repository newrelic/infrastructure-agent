// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmdrequest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

func Test_NewHandleFn_ChildInheritsParentUser(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		parentUser string
		command    string
		args       []string
	}{
		{
			name:       "explicit command inherits restricted parent user",
			parentUser: "nobody",
			command:    "/bin/sh",
			args:       []string{"-c", "id"},
		},
		{
			name:       "integration-name lookup inherits restricted parent user",
			parentUser: "nobody",
			command:    "",
			args:       []string{"--foo"},
		},
		{
			name:       "unrestricted parent produces unrestricted child",
			parentUser: "",
			command:    "/bin/sh",
			args:       []string{"-c", "id"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var lookup integration.InstancesLookup

			lookup.ByName = func(_ string) (string, error) {
				return "/path/to/nri-lookup", nil
			}

			var logger log.Entry

			definitionQueue := make(chan integration.Definition, 1)
			handleFn := NewHandleFn(definitionQueue, lookup, logger)

			var cmd protocol.CmdRequestV1Cmd

			cmd.Name = "child"
			cmd.Command = testCase.command
			cmd.Args = testCase.args

			var crBatch protocol.CmdRequestV1

			crBatch.CommandRequestVersion = "1"
			crBatch.Commands = []protocol.CmdRequestV1Cmd{cmd}

			var parentDefinition integration.Definition

			parentDefinition.ExecutorConfig.User = testCase.parentUser

			handleFn(crBatch, parentDefinition)

			require.Len(t, definitionQueue, 1)

			def := <-definitionQueue
			assert.Equal(t, testCase.parentUser, def.ExecutorConfig.User)
		})
	}
}
