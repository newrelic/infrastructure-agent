// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v3legacy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	nginxApacheFolder    = "./fixtures/nginxapache"
	cassandraMysqlFolder = "./fixtures/cassandramysql"
	apache               = "com.newrelic.apache"
	cassandra            = "com.newrelic.cassandra"
	mysql                = "com.newrelic.mysql"
	nginx                = "com.newrelic.nginx"
)

func TestNewDefinitionsRepo(t *testing.T) {
	// GIVEN a set of definitions folders
	// WHEN they are loaded into a DefinitionsRepo
	dr := NewDefinitionsRepo(LegacyConfig{
		DefinitionFolders: []string{nginxApacheFolder, cassandraMysqlFolder},
	})

	// THEN the DefinitionsRepo contains all the definitions
	require.Contains(t, dr.Definitions, apache)
	require.Contains(t, dr.Definitions, cassandra)
	require.Contains(t, dr.Definitions, mysql)
	require.Contains(t, dr.Definitions, nginx)

	// AND they contain the corresponding commands configuration
	assert.Equal(t, nginxApacheFolder, dr.Definitions[apache].Dir)
	assert.Equal(t, cassandraMysqlFolder, dr.Definitions[cassandra].Dir)
	assert.Equal(t, cassandraMysqlFolder, dr.Definitions[mysql].Dir)
	assert.Equal(t, nginxApacheFolder, dr.Definitions[nginx].Dir)

	// e.g. apache has loaded the "metrics" command
	require.Contains(t, dr.Definitions[apache].Definition.Commands, "metrics")
	metricsCommand := dr.Definitions[apache].Definition.Commands["metrics"]
	assert.Equal(t, []string{"./bin/nr-apache", "--metrics"}, metricsCommand.Command)
	assert.EqualValues(t, 15, metricsCommand.Interval)

	// e.g. cassandra has loaded the "inventory" command
	require.Contains(t, dr.Definitions[cassandra].Definition.Commands, "inventory")
	inventoryCommand := dr.Definitions[cassandra].Definition.Commands["inventory"]
	assert.Equal(t, "config/cassandra", inventoryCommand.Prefix)
	assert.Equal(t, []string{"./bin/nr-cassandra", "--inventory"}, inventoryCommand.Command)
	assert.EqualValues(t, 60, inventoryCommand.Interval)
}
