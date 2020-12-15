// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a definition entry with no discovery sources
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.BasicCmd),
	}, ErrLookup, nil, nil)
	require.NoError(t, err)

	// WHEN it is executed
	outs, err := def.Run(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, outs, 1)

	// THEN returns normally, forwarding the Standard Output&error
	assert.NoError(t, testhelp.ChannelErrClosed(outs[0].Receive.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(outs[0].Receive.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(outs[0].Receive.Stderr))
}

func TestRun_NoDiscovery(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a definition entry with discovery sources
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.BasicCmd),
		Env: map[string]string{
			"CONFIG": "${discovery.foo}",
		},
	}, ErrLookup, nil, nil)
	require.NoError(t, err)

	// WHEN the def is executed with no discovery matches
	outs, err := def.Run(context.Background(), &databind.Values{}, nil, nil)
	require.NoError(t, err)

	// THEN no tasks are executed
	assert.Empty(t, outs)
}

func TestRun_Discovery(t *testing.T) {
	defer leaktest.Check(t)()

	if runtime.GOOS == "windows" {
		t.Skip("there is a problem when executing directly powershell with environment variables")
	}
	// GIVEN a definition entry with discoverable configuration
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.BasicCmd, "${argument}"),
		Env: map[string]string{
			"PREFIX": "${prefix}",
		},
	}, ErrLookup, nil, nil)
	require.NoError(t, err)

	// WHEN the def is executed with different discovery matches
	vals := databind.NewValues(nil,
		databind.NewDiscovery(data.Map{"prefix": "hello", "argument": "world"}, data.InterfaceMap{"special": true, "label.one": "one"}, nil),
		databind.NewDiscovery(data.Map{"prefix": "bye", "argument": "people"}, data.InterfaceMap{"special": false, "label.two": "two"}, nil),
		databind.NewDiscovery(data.Map{"prefix": "kon", "argument": "nichiwa"}, data.InterfaceMap{"other_tag": "true", "label.tree": "three"}, nil),
	)
	outs, err := def.Run(context.Background(), &vals, nil, nil)
	require.NoError(t, err)
	require.Len(t, outs, 3)

	// THEN the tasks are executed with the given configuration
	assert.NoError(t, testhelp.ChannelErrClosed(outs[0].Receive.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(outs[0].Receive.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(outs[0].Receive.Stderr))
	assert.Equal(t, "hello-world", testhelp.ChannelRead(outs[0].Receive.Stdout))
	assert.Equal(t, data.Map{"label.one": "one", "special": "true"}, outs[0].ExtraLabels)

	assert.NoError(t, testhelp.ChannelErrClosed(outs[1].Receive.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(outs[1].Receive.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(outs[1].Receive.Stderr))
	assert.Equal(t, "bye-people", testhelp.ChannelRead(outs[1].Receive.Stdout))
	assert.Equal(t, data.Map{"label.two": "two", "special": "false"}, outs[1].ExtraLabels)

	assert.NoError(t, testhelp.ChannelErrClosed(outs[2].Receive.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(outs[2].Receive.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(outs[2].Receive.Stderr))
	assert.Equal(t, "kon-nichiwa", testhelp.ChannelRead(outs[2].Receive.Stdout))
	assert.Equal(t, data.Map{"label.tree": "three", "other_tag": "true"}, outs[2].ExtraLabels)
}

func TestRun_CmdSlice(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a definition entry whose parameters are specified as a command array
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.CommandSlice(fixtures.BasicCmd, "argument"),
	}, ErrLookup, nil, nil)
	require.NoError(t, err)

	// WHEN the def is executed
	outs, err := def.Run(context.Background(), &databind.Values{}, nil, nil)
	require.NoError(t, err)
	require.Len(t, outs, 1)

	// THEN the tasks are executed with the given configuration
	assert.NoError(t, testhelp.ChannelErrClosed(outs[0].Receive.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(outs[0].Receive.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(outs[0].Receive.Stderr))
	assert.Equal(t, "-argument", testhelp.ChannelRead(outs[0].Receive.Stdout))
}

func TestRun_CancelPropagation(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a definition entry with discoverable configuration
	// that is executed with different discovery matches
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.BlockedCmd, "-f", "${argument}"),
	}, ErrLookup, nil, nil)
	require.NoError(t, err)
	vals := databind.NewValues(nil,
		databind.NewDiscovery(data.Map{"argument": "world"}, nil, nil),
		databind.NewDiscovery(data.Map{"argument": "people"}, nil, nil),
		databind.NewDiscovery(data.Map{"argument": "nichiwa"}, nil, nil),
	)

	parentContext, cancel := context.WithCancel(context.Background())
	outs, err := def.Run(parentContext, &vals, nil, nil)
	require.NoError(t, err)
	require.Len(t, outs, 3)

	// WHEN the tasks are running
	for _, out := range outs {
		assert.Equal(t, "starting", testhelp.ChannelRead(out.Receive.Stdout))
		assert.Error(t, testhelp.ChannelErrClosedTimeout(out.Receive.Errors, 100*time.Millisecond))
	}

	// AND they are cancelled
	cancel()

	// THEN all the subtasks have reported errors
	var openCh bool
	for _, out := range outs {
		err := testhelp.ChannelErrClosed(out.Receive.Errors)
		assert.Error(t, err)
		assert.NotEqual(t, err, testhelp.ErrChannelTimeout)

		// AND channels are closed
		_, openCh = <-out.Receive.Stdout
		assert.False(t, openCh)
		_, openCh = <-out.Receive.Stderr
		assert.False(t, openCh)

		// AND ctx has been canceled
		_, openCh = <-out.Receive.Done
		assert.False(t, openCh)
	}
}

func TestRun_CancelPropagationWithoutReads(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a definition run
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.BlockedCmd),
	}, ErrLookup, nil, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	outs, err := def.Run(ctx, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, outs, 1)

	// AND context is cancelled
	cancel()

	// THEN all the subtasks have reported errors
	var openCh bool
	for _, out := range outs {
		err := testhelp.ChannelErrClosed(out.Receive.Errors)
		assert.Error(t, err)
		assert.NotEqual(t, err, testhelp.ErrChannelTimeout)

		// AND channels are closed
		_, openCh = <-out.Receive.Stdout
		assert.False(t, openCh)
		_, openCh = <-out.Receive.Stderr
		assert.False(t, openCh)

		// AND ctx has been canceled
		_, openCh = <-out.Receive.Done
		assert.False(t, openCh)
	}
}

func TestRun_Cancel_Partial(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a definition entry with discoverable configuration
	// that is executed with different discovery matches
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command("${script}"),
	}, ErrLookup, nil, nil)
	require.NoError(t, err)
	vals := databind.NewValues(nil,
		databind.NewDiscovery(data.Map{"script": string(fixtures.BasicCmd)}, nil, nil),
		databind.NewDiscovery(data.Map{"script": string(fixtures.BlockedCmd)}, nil, nil),
	)

	parentContext, cancel := context.WithCancel(context.Background())
	outs, err := def.Run(parentContext, &vals, nil, nil)
	require.NoError(t, err)
	require.Len(t, outs, 2)

	// WHEN the tasks are running
	assert.Equal(t, "stdout line", testhelp.ChannelRead(outs[0].Receive.Stdout))
	assert.Equal(t, "starting", testhelp.ChannelRead(outs[1].Receive.Stdout))
	assert.Error(t, testhelp.ChannelErrClosedTimeout(outs[1].Receive.Errors, 100*time.Millisecond))

	// AND they are cancelled
	cancel()

	// THEN only the non-finished tasks have been cancelled
	assert.NoError(t, testhelp.ChannelErrClosed(outs[0].Receive.Errors))
	assert.Error(t, testhelp.ChannelErrClosedTimeout(outs[1].Receive.Errors, 100*time.Millisecond))
}

func TestRun_Directory(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a definition that is located in a non-current directory
	tmpDir, err := ioutil.TempDir("", "script")
	require.NoError(t, err)
	script, err := ioutil.ReadFile(string(fixtures.BasicCmd))
	require.NoError(t, err)
	scriptFile := filepath.Base(string(fixtures.BasicCmd))
	require.NoError(t, ioutil.WriteFile(filepath.Join(tmpDir, scriptFile), script, os.ModePerm))

	// GIVEN a definition entry with a user-provided working directory
	// that invokes a script with a relative path
	currentpath := "./"
	if runtime.GOOS == "windows" {
		currentpath = ".\\"
	}
	def, err := NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(testhelp.Script(currentpath + scriptFile)),
		WorkDir:      tmpDir,
	}, ErrLookup, nil, nil)
	require.NoError(t, err)

	// WHEN it is executed
	outs, err := def.Run(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, outs, 1)

	// THEN returns normally, forwarding the Standard Output&error
	assert.NoError(t, testhelp.ChannelErrClosed(outs[0].Receive.Errors))
	assert.Equal(t, "stdout line", testhelp.ChannelRead(outs[0].Receive.Stdout))
	assert.Equal(t, "error line", testhelp.ChannelRead(outs[0].Receive.Stderr))
}

func TestRun_RemoveExternalConfig(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN an integration with an external configuration file

	configEntry := config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.FileContentsWithArgCmd, "${config.path}"),
		Config:       "${discovery.ip}",
	}
	config, err := LoadConfigTemplate(configEntry.TemplatePath, configEntry.Config)
	require.NoError(t, err)

	def, err := NewDefinition(configEntry, ErrLookup, nil, config)
	require.NoError(t, err)

	// WHEN the integration has been properly executed
	vals := databind.NewValues(nil,
		databind.NewDiscovery(data.Map{"discovery.ip": "1.2.3.4"}, nil, nil),
		databind.NewDiscovery(data.Map{"discovery.ip": "5.6.7.8"}, nil, nil),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// (spy function to get which files have been created)
	var createdConfigs []string
	def.newTempFile = func(template []byte) (string, error) {
		path, err := newTempFile(template)
		if err == nil {
			createdConfigs = append(createdConfigs, path)
		}
		return path, err
	}
	outputs, err := def.Run(ctx, &vals, nil, nil)
	require.NoError(t, err)
	require.Len(t, outputs, 2)
	require.Len(t, createdConfigs, 2)

	timeout := time.After(10 * time.Second)
	for _, out := range outputs {
		select {
		case <-out.Receive.Done:
		case <-timeout:
			require.FailNow(t, "timeout waiting for the integrations to finish")
		}
	}

	// THEN the external configuration file has been removed
	testhelpers.Eventually(t, 5*time.Second, func(t require.TestingT) {
		for _, path := range createdConfigs {
			_, err := os.Stat(path)
			assert.Truef(t, os.IsNotExist(err), "expecting file %q to not exist. Error: %v", path, err)
		}
	})
}
