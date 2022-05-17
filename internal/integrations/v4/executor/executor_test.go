// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package executor

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/constants"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnable_CLI_Execute(t *testing.T) {
	defer leaktest.Check(t)()

	// testing many times the set to be sure there are no missed lines/sync problems
	for i := 0; i < 20; i++ {
		runtime.GOMAXPROCS(i % 4) // it tests several concurrency configurations

		// GIVEN a runnable instance that points to a working executable
		r := FromCmdSlice(testhelp.Command(fixtures.BasicCmd), execConfig(t))

		// WHEN it is executed
		to := r.Execute(context.Background(), nil, nil)

		// THEN no errors are returned
		assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))

		// AND standard output lines are returned
		assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))

		// AND error lines are returned
		assert.Equal(t, "error line", testhelp.ChannelRead(to.Stderr))
	}
}

func TestRunnable_CLI_Execute_with_spaces(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a runnable instance that points to a working executable
	r := FromCmdSlice(testhelp.Command(fixtures.BasicCmdWithSpace), execConfig(t))

	// WHEN it is executed
	to := r.Execute(context.Background(), nil, nil)

	// THEN no errors are returned
	assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))

	// AND standard output lines are returned
	assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))

	// AND error lines are returned
	assert.Equal(t, "error line", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_Execute_WithUser(t *testing.T) {
	t.Skip()

	defer leaktest.Check(t)()

	// GIVEN a runnable instance that is run with a given user
	c, err := user.Current()
	require.NoError(t, err)
	cfg := execConfig(t)
	cfg.User = c.Username
	r := FromCmdSlice(testhelp.Command(fixtures.BasicCmd), cfg)

	// WHEN it is executed
	to := r.Execute(context.Background(), nil, nil)

	// THEN no errors are returned
	assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))

	// AND standard output lines are returned
	assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))

	// AND error lines are returned
	assert.Equal(t, "error line", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_Execute_WithArgs(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a working runnable that is configured with CLI arguments
	cfg := execConfig(t)
	r := FromCmdSlice(testhelp.Command(fixtures.BasicCmd, "world"), cfg)

	to := r.Execute(context.Background(), nil, nil)
	assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))
	assert.Equal(t, "-world", testhelp.ChannelRead(to.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_Execute_WithArgs_WithEnv(t *testing.T) {
	defer leaktest.Check(t)()

	if runtime.GOOS == "windows" {
		t.Skip("there is a problem when executing directly powershell with environment variables")
	}
	// GIVEN a working runnable that is configured with CLI arguments and env vars
	cfg := execConfig(t)
	cfg.Environment = map[string]string{"PREFIX": "hello"}
	r := FromCmdSlice(testhelp.Command(fixtures.BasicCmd, "world"), cfg)

	to := r.Execute(context.Background(), nil, nil)
	assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))
	assert.Equal(t, "hello-world", testhelp.ChannelRead(to.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_Execute_Error(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a runnable instance that fails
	r := FromCmdSlice(testhelp.Command(fixtures.ErrorCmd), execConfig(t))

	// WHEN it is executed
	to := r.Execute(context.Background(), nil, nil)

	// THEN an error is returned
	assert.Error(t, testhelp.ChannelErrClosed(to.Errors))

	// AND standard output lines are anyway returned
	assert.Equal(t, "starting", testhelp.ChannelRead(to.Stdout))

	// AND error lines are anyway returned
	assert.Equal(t, "very bad error", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_Execute_FDsNotLeakedWhenFileDoesNotExist(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a runnable instance does not exist
	r := FromCmdSlice(testhelp.Command("../fixtures/i_do_not_exist.sh"), execConfig(t))

	// WHEN it is executed
	to := r.Execute(context.Background(), nil, nil)

	// AND it finished
	<-to.Done

	// THEN an error should be provided to be handled later on
	// (handlers might be: runner.handleErrors or Supervisor.handleErrs)
	err := testhelp.ChannelErrClosed(to.Errors)
	_, ok := err.(*exec.ExitError)
	require.True(t, ok)
	// AND stdout channel should be closed
	if runtime.GOOS != "windows" {
		// Win returns garbage messages through stdout when no file is found
		testhelp.AssertChanIsClosed(t, to.Stdout)
	}
	// AND stderr channel needs to be drained
	// (handlers might be: runner.handleStderr or Supervisor.handleStdErr)
	assert.NotEmpty(t, string(<-to.Stderr))
	testhelp.AssertChanIsClosed(t, to.Stderr)
}

func TestRunnable_Execute_Blocked(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a blocked runnable instance
	cfg := execConfig(t)

	ctx, cancel := context.WithCancel(context.Background())
	r := FromCmdSlice(testhelp.Command(fixtures.BlockedCmd), cfg)

	// THAT is normally working
	to := r.Execute(ctx, nil, nil)
	assert.Equal(t, "starting", testhelp.ChannelRead(to.Stdout))
	assert.Error(t, testhelp.ChannelErrClosedTimeout(to.Errors, 100*time.Millisecond))

	// WHEN the running context is cancelled
	cancel()

	// THEN the runnable has been interrupted, returning error
	err := testhelp.ChannelErrClosed(to.Errors)
	assert.Error(t, err)
	assert.NotEqual(t, testhelp.ErrChannelTimeout, err)
}

func TestNoRaces(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	defer leaktest.Check(t)()

	bytes := make([]byte, 1000000)
	for i := range bytes {
		bytes[i] = 'a'
	}
	hugeLine := string(bytes)

	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cmd := FromCmdSlice([]string{"echo", hugeLine}, &Config{})
		go cmd.Execute(ctx, nil, nil)
		cancel()
	}
}

func TestRunnable_Execute_Verbose(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a runnable instance that points to a working executable
	r := FromCmdSlice(testhelp.Command(fixtures.IntegrationVerboseScript), execConfig(t))

	// GIVEN verbose set to 1
	ctx := context.WithValue(context.Background(), constants.EnableVerbose, 1)

	// WHEN it is executed
	to := r.Execute(ctx, nil, nil)

	// THEN no errors are returned
	assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))

	// AND standard output lines are returned
	assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))

	// AND error lines are returned
	assert.Equal(t, "VERBOSE=1", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_Execute_VerboseFalse(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a runnable instance that points to a working executable
	r := FromCmdSlice(testhelp.Command(fixtures.IntegrationVerboseScript), execConfig(t))

	// GIVEN verbose set to 0
	ctx := context.WithValue(context.Background(), constants.EnableVerbose, 0)

	// WHEN it is executed
	to := r.Execute(ctx, nil, nil)

	// THEN no errors are returned
	assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))

	// AND standard output lines are returned
	assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))

	// AND error lines are returned
	assert.Equal(t, "VERBOSE=", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_Execute_NoVerboseSet(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a runnable instance that points to a working executable
	r := FromCmdSlice(testhelp.Command(fixtures.IntegrationVerboseScript), execConfig(t))

	// WHEN it is executed
	to := r.Execute(context.Background(), nil, nil)

	// THEN no errors are returned
	assert.NoError(t, testhelp.ChannelErrClosed(to.Errors))

	// AND standard output lines are returned
	assert.Equal(t, "stdout line", testhelp.ChannelRead(to.Stdout))

	// AND error lines are returned
	assert.Equal(t, "VERBOSE=", testhelp.ChannelRead(to.Stderr))
}

func TestRunnable_BuildCommandWithNriaPassthroughEnvironment(t *testing.T) {
	defer leaktest.Check(t)()

	tests := []struct {
		name           string
		cfgEnv         map[string]string
		osEnv          map[string]string
		passthrough    []string
		expectedCmdEnv []string
	}{
		{
			name:           "no passthrough variables",
			cfgEnv:         map[string]string{"PREFIX": "hello"},
			osEnv:          map[string]string{"SOME_VAR": "some value", "ANOTHER_VAR": "another value"},
			passthrough:    nil,
			expectedCmdEnv: []string{"PREFIX=hello"},
		},
		{
			name:           "one passthrough variable",
			cfgEnv:         map[string]string{"PREFIX": "hello"},
			osEnv:          map[string]string{"SOME_VAR": "some value", "ANOTHER_VAR": "another value"},
			passthrough:    []string{"ANOTHER_VAR"},
			expectedCmdEnv: []string{"PREFIX=hello", "ANOTHER_VAR=another value"},
		},
		{
			name:           "one duplicated passthrough variable",
			cfgEnv:         map[string]string{"PREFIX": "hello"},
			osEnv:          map[string]string{"SOME_VAR": "some value", "ANOTHER_VAR": "another value"},
			passthrough:    []string{"ANOTHER_VAR", "ANOTHER_VAR"},
			expectedCmdEnv: []string{"PREFIX=hello", "ANOTHER_VAR=another value"},
		},
		{
			name:           "multiple passthrough variable",
			cfgEnv:         map[string]string{"PREFIX": "hello"},
			osEnv:          map[string]string{"SOME_VAR": "some value", "ANOTHER_VAR": "another value"},
			passthrough:    []string{"SOME_VAR", "ANOTHER_VAR"},
			expectedCmdEnv: []string{"PREFIX=hello", "SOME_VAR=some value", "ANOTHER_VAR=another value"},
		},
		{
			name:           "passthrough variable has precedence over integration one",
			cfgEnv:         map[string]string{"PREFIX": "hello", "SOME_VAR": "integration value"},
			osEnv:          map[string]string{"SOME_VAR": "some value", "ANOTHER_VAR": "another value"},
			passthrough:    []string{"SOME_VAR", "ANOTHER_VAR"},
			expectedCmdEnv: []string{"PREFIX=hello", "SOME_VAR=some value", "ANOTHER_VAR=another value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN a working runnable that is configured with CLI arguments and env vars AND passthrough env variables
			cfg := execConfig(t)
			cfg.Environment = tt.cfgEnv
			cfg.Passthrough = tt.passthrough
			r := FromCmdSlice(testhelp.Command(fixtures.BasicCmd, "world"), cfg)

			// AND os ENV variables are set
			for k, v := range tt.osEnv {
				err := os.Setenv(k, v)
				assert.Nil(t, err)
			}
			// WHEN building the command
			cmd := r.buildCommand(context.Background())

			// THEN only os env variables present in passthrough should be passed to command
			// and have precedence over the integration ones
			assert.ElementsMatch(t, tt.expectedCmdEnv, cmd.Env)
		})
	}
}

func execConfig(t require.TestingT) *Config {
	d, err := os.Getwd()
	require.NoError(t, err)
	return &Config{
		Directory:   d,
		Environment: nil,
	}
}
