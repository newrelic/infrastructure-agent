// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package command

import (
	"encoding/json"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunFunc(t *testing.T) {
	//t.Parallel()
	envVarVal := "some random value"
	rndEnv := os.Environ()[0]
	rndEnvIdx := strings.Index(rndEnv, "=")

	echoJson := `[
{\"variables\":{\"ip\":\"1.2.3.5\",\"port\":101,\"version\":3},
\"metricAnnotations\":{\"test\":true,\"test_environment_var\":\"${test_environment_var}\",\"test_name\":\"TestRunFunc\", \"%s\":\"${%s}\", \"a_map\":{\"something\":\"important\",\"number\":1}},
\"entityRewrites\":[]},
{\"variables\":{\"ip\":\"1.2.3.4\",\"port\":1337,\"version\":2.1},
\"metricAnnotations\":{\"test\":true,\"test_name\":\"TestRunFunc\"},
\"entityRewrites\":[]}]`
	vars := []data.GenericDiscovery{
		{
			Variables: data.InterfaceMap{
				"ip":      "1.2.3.5",
				"port":    float64(101), // Done to fix equals
				"version": float64(3),
			},
			Annotations: data.InterfaceMap{
				"test":                 true,
				"test_name":            "TestRunFunc",
				"test_environment_var": envVarVal,
				rndEnv[0:rndEnvIdx]:    rndEnv[rndEnvIdx+1:],
				"a_map": map[string]interface{}{
					"number":    float64(1),
					"something": "important",
				},
			},
			EntityRewrites: []data.EntityRewrite{},
		},
		{
			Variables: data.InterfaceMap{
				"ip":      "1.2.3.4",
				"port":    float64(1337),
				"version": 2.1,
			},
			Annotations: data.InterfaceMap{
				"test":      true,
				"test_name": "TestRunFunc",
			},
			EntityRewrites: []data.EntityRewrite{},
		},
	}

	bytes, err := json.Marshal(vars)
	require.NoError(t, err)

	d := discovery.Command{
		Exec: discovery.ShlexOpt{"sh", "-c", "echo \"" + fmt.Sprintf(echoJson, rndEnv[0:rndEnvIdx], rndEnv[0:rndEnvIdx]) + "\""},
		Environment: map[string]string{
			"test_environment_var": envVarVal,
		},
	}
	results, err := run(d)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, vars, results)
	t.Log(results)
	t.Log(string(bytes))
}

func TestRunFunc_missing_command(t *testing.T) {
	t.Parallel()

	d := discovery.Command{
		Exec: discovery.ShlexOpt{"none-existing-command"},
	}

	results, err := run(d)
	require.Error(t, err)
	assert.Empty(t, results)
}

func TestRunFunc_silent_error(t *testing.T) {
	t.Parallel()

	d := discovery.Command{
		Exec: discovery.ShlexOpt{"sh", "-c", "echo 'this is bad' >&2; exit 101"},
	}

	results, err := run(d)
	require.Error(t, err)
	assert.Empty(t, results)
	assert.EqualError(t, err, "this is bad\nexit status 101")
}

func TestRunFunc_timeout(t *testing.T) {
	t.Parallel()

	d := discovery.Command{
		Exec:    discovery.ShlexOpt{"sleep", "1"},
		Timeout: time.Millisecond,
	}

	results, err := run(d)
	require.Error(t, err)
	assert.Empty(t, results)
	assert.EqualError(t, err, "command timed out")
}
