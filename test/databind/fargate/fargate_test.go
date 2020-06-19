// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build slow

package fargate_test

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	test "github.com/newrelic/infrastructure-agent/test/databind"
)

func TestIntegrationFargate(t *testing.T) {
	if err := test.ComposeUp("./docker-compose.yml"); err != nil {
		log.Println("error on compose-up: ", err.Error())
		os.Exit(-1)
	}
	defer test.ComposeDown("./docker-compose.yml")

	out, err := test.Exec("fargate_test_executor_1",
		"go", "test", "-v", "--tags=fargate_inside", "-run", "^TestFargate", "github.com/newrelic/infrastructure-agent/test/databind/...")
	t.Log(out)
	require.NoError(t, err, "inside tests failed. Look previous output for more info")
}
