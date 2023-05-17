// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/constants"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/v3legacy"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotest "gotest.tools/assert"
)

var invalidFile = `--
	
`

var v3File = `---
integration_name: some.integration.name
instances:
  - name: We don't' care about this
    command: because it will be ignored
`

func getExe(exec config.ShlexOpt) string {
	return strings.Join(exec, " ")
}

var v4File = `---
integrations:
  - name: hello-test
    exec: ` + getExe(testhelp.GoRun(fixtures.SimpleGoFile, "hello")) + `
  - name: goodbye-test
    exec: ` + getExe(testhelp.GoRun(fixtures.SimpleGoFile, "goodbye")) + "\n"

var v4FileWithConfigYAML = `---
integrations:
  - name: config-test
    exec: ` + getExe(testhelp.GoRun(fixtures.ValidYAMLGoFile, "${config.path}")) + `
    config:
      event_type: YAMLEvent
      map:
        key: value
      array:
        - item1
        - item2
`

var v4LongTimeConfig = `---
integrations:
  - name: longtime
    exec: ` + getExe(testhelp.GoRun(fixtures.LongTimeGoFile, "longtime")) + "\n"

// for Hot reload test, you only have to append a line with an extra argument
// to change the integration configuration
var v4AppendableConfig = `---
integrations:
  - name: hotreload-test
    exec:
      - ` + testhelp.GoCommand() + `
      - run
      - ` + string(fixtures.LongTimeGoFile) + "\n"

var v4FileWithNriDockerNameAndDockerFF = `---
integrations:
  - name: nri-docker
    when:
      feature: docker_enabled
    exec: ` + getExe(testhelp.GoRun(fixtures.SimpleGoFile, "hello"))

var v4FileWithContinuousNriDocker = `---
integrations:
  - name: nri-docker
    when:
      feature: docker_enabled
    exec:
      - ` + testhelp.GoCommand() + `
      - run
      - ` + string(fixtures.LongTimeGoFile) + "\n"

var v4FileWithWhen = `---
integrations:
  - name: hello-test
    exec: ` + getExe(testhelp.GoRun(fixtures.SimpleGoFile, "hello")) + `
    when:
      file_exists: %s
`

var v4VerboseCheck = `---
integrations:
  - name: verbose-check
    exec: ` + getExe(testhelp.GoRun(fixtures.EnvironmentGoFile)) + `
    env:
      THIS_IS_A_TEST: true
      GOTMPDIR: %s
      GOCACHE: %s
`

var v4CmdRequest = `---
integrations:
  - name: cmdreq
    exec: ` + getExe(testhelp.GoRun(fixtures.CmdReqGoFile)) + "\n"

var v4CfgReqV3Payload = `---
integrations:
  - name: cfgreq
    env:
      STDOUT_TYPE: v3
    exec: ` + getExe(testhelp.GoRun(fixtures.CfgReqGoFile)) + "\n"

var v4CfgReqV4Payload = `---
integrations:
  - name: cfgreq
    env:
      STDOUT_TYPE: v4
    exec: ` + getExe(testhelp.GoRun(fixtures.CfgReqGoFile)) + "\n"

var v4CfgReqRecursive = `---
integrations:
  - name: cfgreq-recursive
    env:
      STDOUT_TYPE: cfgreq
    exec: ` + getExe(testhelp.GoRun(fixtures.CfgReqGoFile)) + "\n"

//nolint:gochecknoglobals
var passthroughEnv = []string{"GOCACHE", "GOPATH", "HOME", "PATH", "LOCALAPPDATA", "GOTMPDIR"}

var (
	definitionQ          = make(chan integration.Definition, 1000)
	configEntryQ         = make(chan configrequest.Entry, 1000)
	terminateDefinitionQ = make(chan string, 1000)
)

func TestManager_StartIntegrations(t *testing.T) {
	// GIVEN a set of configuration files
	dir, err := tempFiles(map[string]string{
		"v4-integrations.yaml": v4File,
		"v3-config.yaml":       v3File, // it will be ignored
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager loads and executes the integrations in the folder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN all the v4 integrations start emitting data
	metric := expectOneMetric(t, emitter, "hello-test")
	require.Equal(t, "hello", metric["value"])

	metric = expectOneMetric(t, emitter, "goodbye-test")
	require.Equal(t, "goodbye", metric["value"])
}

func removeTempFiles(t *testing.T, dir string) {
	func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Log(err)
		}
	}()
}

func TestManager_IntegrationProtocolV4(t *testing.T) {
	dir, err := tempFiles(map[string]string{
		"kubernetes-like.yml": `
integrations:
  - name: nri-kubernetes
    exec: ` + getExe(testhelp.GoRun(fixtures.HugeGoFile)),
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager loads and executes the integration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN all the emitted data is received
	metric := expectOneMetric(t, emitter, "nri-kubernetes")
	require.Equal(t, "K8sSchedulerSample", metric["event_type"])
}

func TestManager_ProtocolV4(t *testing.T) {
	// GIVEN an integration returning a protocol v4 payload
	dir, err := tempFiles(map[string]string{
		"protocol_v4.yml": `
integrations:
  - name: nri-protocol-v4
    exec: ` + getExe(testhelp.GoRun(fixtures.ProtocolV4GoFile)),
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager loads and executes the integration
	ctx, cancel := context.WithCancel(context.Background())

	finish := make(chan struct{})

	go func() {
		mgr.Start(ctx)
		close(finish)
	}()

	// THEN emitted metrics are received (gauge, count & summary)
	_ = expectNMetrics(t, emitter, "nri-protocol-v4", 3)
	cancel()

	<-finish
}

func TestManager_SkipLoadingV3IntegrationsWithNoWarnings(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	// GIVEN a set of configuration files
	dir, err := tempFiles(map[string]string{
		"v4-integrations.yaml": v4File,
		"v3-config.yaml":       v3File, // it will be ignored
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	_ = NewManager(ManagerConfig{ConfigPaths: []string{dir}}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// THEN no long entries found
	for i := range hook.AllEntries() {
		fmt.Println(hook.AllEntries()[i]) // Use stdout as logger is in discard mode and we never run tests in verbose
	}
	assert.Empty(t, hook.AllEntries())
}

func TestManager_LogWarningForInvalidYaml(t *testing.T) {
	hook := new(test.Hook)
	log.AddHook(hook)

	// GIVEN a set of configuration files
	dir, err := tempFiles(map[string]string{
		"v4-integrations.yaml": invalidFile,
		"v3-config.yaml":       v3File, // it will be ignored
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	_ = NewManager(ManagerConfig{ConfigPaths: []string{dir}}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// THEN one long entry found
	require.NotEmpty(t, hook.AllEntries())
	entry := hook.LastEntry()
	assert.Equal(t, "can't load integrations file. This may happen if you are editing a file and saving intermediate changes", entry.Message)
	assert.Equal(t, logrus.WarnLevel, entry.Level)
}

func TestManager_Config_EmbeddedYAML(t *testing.T) {
	// GIVEN an integration configuration that embeds the external config file as a YAML config field
	dir, err := tempFiles(map[string]string{
		"v4-integration.yaml": v4FileWithConfigYAML,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager loads and executes the integrations in the folder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integration has correctly received the embedded yaml as a simple YAML
	// (and we know it because it emits the YAML as a JSON integration)
	metric := expectOneMetric(t, emitter, "config-test")
	assert.Equal(t, "YAMLEvent", metric["event_type"])
	gotest.DeepEqual(t, map[string]interface{}{"key": "value"}, metric["map"])
	gotest.DeepEqual(t, []interface{}{"item1", "item2"}, metric["array"])
}

func TestManager_HotReload_Add(t *testing.T) {
	skipIfWindows(t)
	// GIVEN an integration
	dir, err := tempFiles(map[string]string{
		"integration.yaml": v4AppendableConfig,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THAT is correctly running
	// (the first returned metric value is "first")
	metric := expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "first", metric["value"])
	// (then returns a value passed by argument, or "unset" if not set)
	metric = expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "unset", metric["value"])

	// WHEN we add a new integration file to the directory
	require.NoError(t, ioutil.WriteFile(filepath.Join(dir, "new-integration.yaml"),
		[]byte(v4LongTimeConfig), 0o666))

	// THEN a new integration is started
	metric = expectOneMetric(t, emitter, "longtime")
	require.Equal(t, "first", metric["value"])
	metric = expectOneMetric(t, emitter, "longtime")
	require.Equal(t, "longtime", metric["value"])
}

func TestManager_HotReload_Modify(t *testing.T) {
	skipIfWindows(t)
	// GIVEN an integration
	dir, err := tempFiles(map[string]string{
		"integration.yaml": v4AppendableConfig,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THAT is correctly running
	// (the first returned metric value is "first")
	metric := expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "first", metric["value"])

	// (then returns a value passed by argument, or "unset" if not set)
	metric = expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "unset", metric["value"])

	// WHEN we modify the integration file at runtime
	require.NoError(t, fileAppend(
		filepath.Join(dir, "integration.yaml"),
		"      - modifiedValue\n"))

	// THEN the integration is restarted
	testhelpers.Eventually(t, 5*time.Second, func(t require.TestingT) {
		// waiting to empty the previous process queue and receive a "first" value again
		firstMetric := expectOneMetric(t, emitter, "hotreload-test")
		require.Equal(t, "first", firstMetric["value"])
	})
	// AND the integration reflects the changes in the configuration file
	metric = expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "modifiedValue", metric["value"])
}

// this test is used to make sure we see file changes on K8s
func TestManager_HotReload_ModifyLinkFile(t *testing.T) {
	skipIfWindows(t)
	// GIVEN an integration
	dir, err := tempFiles(map[string]string{
		"integration.yaml": v4AppendableConfig,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	err = os.Rename(filepath.Join(dir, "integration.yaml"), filepath.Join(dir, "first_config"))
	require.NoError(t, err)

	err = os.Link(filepath.Join(dir, "first_config"), filepath.Join(dir, "integration.yaml"))
	require.NoError(t, err)

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THAT is correctly running
	// (the first returned metric value is "first")
	metric := expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "first", metric["value"])

	// (then returns a value passed by argument, or "unset" if not set)
	metric = expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "unset", metric["value"])

	// WHEN we modify the integration file at runtime by changing symlink
	bs, err := ioutil.ReadFile(filepath.Join(dir, "first_config"))
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(dir, "second_config"), bs, 0o644))
	require.NoError(t, fileAppend(
		filepath.Join(dir, "second_config"),
		"      - modifiedValue\n"))
	require.NoError(t,
		os.Remove(filepath.Join(dir, "integration.yaml")))
	require.NoError(t,
		os.Link(filepath.Join(dir, "second_config"), filepath.Join(dir, "integration.yaml")))

	// THEN the integration is restarted
	testhelpers.Eventually(t, 15*time.Second, func(t require.TestingT) {
		// waiting to empty the previous process queue and receive a "first" value again
		metric = expectOneMetric(t, emitter, "hotreload-test")
		require.Equal(t, "first", metric["value"])
	})
	// AND the integration reflects the changes in the configuration file
	metric = expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "modifiedValue", metric["value"])
}

func TestManager_HotReload_Delete(t *testing.T) {
	skipIfWindows(t)
	// GIVEN a set of integrations
	dir, err := tempFiles(map[string]string{
		"integration.yaml":   v4AppendableConfig,
		"to-be-deleted.yaml": v4LongTimeConfig,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THAT are correctly running
	// (the first returned metric value is "first")
	metric := expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "first", metric["value"])
	metric = expectOneMetric(t, emitter, "longtime")
	require.Equal(t, "first", metric["value"])
	// (then return a value passed by argument, or "unset" if not set)
	metric = expectOneMetric(t, emitter, "hotreload-test")
	require.Equal(t, "unset", metric["value"])
	metric = expectOneMetric(t, emitter, "longtime")
	require.Equal(t, "longtime", metric["value"])

	// WHEN we delete an integration file at runtime
	require.NoError(t, os.Remove(filepath.Join(dir, "to-be-deleted.yaml")))

	// THEN the integration eventually stops reporting
	testhelpers.Eventually(t, 5*time.Second, func(t require.TestingT) {
		require.NoError(t, emitter.ExpectTimeout("longtime", 200*time.Millisecond))
	})
	// and does not report ever again
	require.NoError(t, emitter.ExpectTimeout("longtime", 100*time.Millisecond))
}

func TestManager_PassthroughEnv(t *testing.T) {
	// GIVEN an integration
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(t, err)
	defer removeTempFiles(t, niDir)
	execPath := filepath.Join(niDir, "nri-simple"+fixtures.CmdExtension)
	require.NoError(t, testhelp.GoBuild(fixtures.SimpleGoFile, execPath))
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `
integrations:
  - name: nri-simple
`,
	})
	require.NoError(t, err)

	// WHEN the manager sets the PassthroughEnvironment configuration to an existing variable
	unset := testhelpers.Setenv("VALUE", "hello-there")
	defer unset()
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{configDir},
		DefinitionFolders:      []string{niDir},
		PassthroughEnvironment: []string{"VALUE"},
	}, config.NewPathLoader(), emitter, instancesLookupReturning(execPath), definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integrations take the configuration from the environment
	metric := expectOneMetric(t, emitter, "nri-simple")
	require.Equal(t, "hello-there", metric["value"])
}

func TestManager_PassthroughEnv_Priorities(t *testing.T) {
	// GIVEN an integration that configures an environment variables
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(t, err)
	defer removeTempFiles(t, niDir)

	execPath := filepath.Join(niDir, "nri-simple"+fixtures.CmdExtension)
	require.NoError(t, testhelp.GoBuild(fixtures.SimpleGoFile, execPath))
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `
integrations:
  - name: nri-simple
    env:
      VALUE: value-from-config
`,
	})
	require.NoError(t, err)

	// WHEN the manager that sets the PassthroughEnvironment configuration
	// to a variable that is already defined in the configuration
	unset := testhelpers.Setenv("VALUE", "value-from-env")
	defer unset()

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{configDir},
		DefinitionFolders:      []string{niDir},
		PassthroughEnvironment: []string{"VALUE"},
	}, config.NewPathLoader(), emitter, instancesLookupReturning(execPath), definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the passed-through variable has precedence over the
	// file variable
	metric := expectOneMetric(t, emitter, "nri-simple")
	require.Equal(t, "value-from-env", metric["value"])
}

func TestManager_LegacyIntegrations(t *testing.T) {
	skipIfWindows(t)
	// GIVEN a v3 definitions folder with its compiled binaries
	definitionsDir, err := tempFiles(map[string]string{
		"longtime-definition.yml": fixtures.LongtimeDefinition,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, definitionsDir)
	binDir := filepath.Join(definitionsDir, "bin")
	require.NoError(t, os.Mkdir(binDir, 0o777))
	require.NoError(t, testhelp.GoBuild(fixtures.LongTimeGoFile, filepath.Join(binDir, "longtime"+fixtures.CmdExtension)))

	// AND a v4 configuration folder that references commands from the above definitions
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `
integrations:
  - name: say-hello
    integration_name: com.newrelic.longtime
    command: hello
  - name: say-goodbye
    integration_name: com.newrelic.longtime
    command: use_env
    arguments:
      value: goodbye
`,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, configDir)

	// WHEN the v4 integrations manager loads the legacy definitions
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:       []string{configDir},
		DefinitionFolders: []string{definitionsDir},
	}, config.NewPathLoader(), emitter, instancesLookupLegacy(definitionsDir), definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integrations are properly configured and reporting
	metric := expectOneMetric(t, emitter, "say-hello")
	require.Equal(t, "first", metric["value"])
	metric = expectOneMetric(t, emitter, "say-hello")
	require.Equal(t, "hello", metric["value"])
	metric = expectOneMetric(t, emitter, "say-goodbye")
	require.Equal(t, "first", metric["value"])
	metric = expectOneMetric(t, emitter, "say-goodbye")
	require.Equal(t, "goodbye", metric["value"])
}

func TestManager_LegacyIntegrations_PassthroughEnv(t *testing.T) {
	skipIfWindows(t)
	// GIVEN a v3 definitions folder with its compiled binaries
	definitionsDir, err := tempFiles(map[string]string{
		"longtime-definition.yml": fixtures.LongtimeDefinition,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, definitionsDir)
	binDir := filepath.Join(definitionsDir, "bin")
	require.NoError(t, os.Mkdir(binDir, 0o777))
	require.NoError(t, testhelp.GoBuild(fixtures.LongTimeGoFile, filepath.Join(binDir, "longtime"+fixtures.CmdExtension)))

	// AND a v4 configuration folder that references a command from the above definitions
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `
integrations:
  - name: say-something
    integration_name: com.newrelic.longtime
    command: use_env
`,
	})
	require.NoError(t, err)

	// WHEN the v4 integrations manager loads the legacy definition
	// with passthrough for their configuration
	unset := testhelpers.Setenv("VALUE", "passed-through")
	defer unset()

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{configDir},
		DefinitionFolders:      []string{definitionsDir},
		PassthroughEnvironment: []string{"VALUE"},
	}, config.NewPathLoader(), emitter, instancesLookupLegacy(definitionsDir), definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integrations are properly configured and reporting
	metric := expectOneMetric(t, emitter, "say-something")
	require.Equal(t, "first", metric["value"])
	metric = expectOneMetric(t, emitter, "say-something")
	require.Equal(t, "passed-through", metric["value"])
}

func TestManager_NamedIntegration(t *testing.T) {
	skipIfWindows(t)
	// GIVEN an set of agent directories containing compiled binaries
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(t, err)
	defer removeTempFiles(t, niDir)

	ciDir, err := ioutil.TempDir("", "custom integrations") // using spaces to make sure they are not taken as different arguments
	require.NoError(t, err)
	defer removeTempFiles(t, ciDir)
	execPath1 := filepath.Join(niDir, "nri-longtime"+fixtures.CmdExtension)
	execPath2 := filepath.Join(ciDir, "nri-simple"+fixtures.CmdExtension)
	require.NoError(t, testhelp.GoBuild(fixtures.LongTimeGoFile, execPath1))
	require.NoError(t, testhelp.GoBuild(fixtures.SimpleGoFile, execPath2))

	// AND a v4 configuration file that references the above commands only by name
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `
integrations:
  - name: nri-longtime
  - name: nri-simple       # the 'name' directive does not accept arguments (use 'exec')
    env:                   # but allows using environment variables as configuration
      VALUE: my-value
`,
	})
	require.NoError(t, err)

	// WHEN the v4 integrations manager recognizes the above folders
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:       []string{configDir},
		DefinitionFolders: []string{niDir, ciDir, "unexisting-dir"},
	}, config.NewPathLoader(), emitter, instancesLookupReturning(execPath1, execPath2), definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integrations are properly executed and reporting
	metric := expectOneMetric(t, emitter, "nri-simple")
	require.Equal(t, "my-value", metric["value"])
	metric = expectOneMetric(t, emitter, "nri-longtime")
	require.Equal(t, "first", metric["value"])
	metric = expectOneMetric(t, emitter, "nri-longtime")
	require.Equal(t, "unset", metric["value"])
}

func TestManager_NamedIntegrationWithConfig(t *testing.T) {
	// GIVEN an set of agent directories containing compiled binaries
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(t, err)
	defer removeTempFiles(t, niDir)
	execPath := filepath.Join(niDir, "nri-validyaml"+fixtures.CmdExtension)
	require.NoError(t, testhelp.GoBuild(fixtures.ValidYAMLGoFile, execPath))

	// AND a v4 named integration with an embedded config
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `
integrations:
  - name: nri-validyaml
    config:
      event_type: YAMLEvent
      map:
        hello: foo
`,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, configDir)

	// WHEN the v4 integrations are run
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:       []string{configDir},
		DefinitionFolders: []string{niDir},
	}, config.NewPathLoader(), emitter, instancesLookupReturning(execPath), definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integration has received the YAML by the default environment variable
	metric := expectOneMetric(t, emitter, "nri-validyaml")
	assert.Equal(t, "YAMLEvent", metric["event_type"])
	gotest.DeepEqual(t, map[string]interface{}{"hello": "foo"}, metric["map"])
}

func TestManager_EnableFeature_WhenFeatureOnOHICfgAndAgentCfgIsDisabledAndEnabledFromCmdCh(t *testing.T) {
	// GIVEN a configuration file for an OHI
	dir, err := tempFiles(map[string]string{
		"foo.yaml": v4FileWithNriDockerNameAndDockerFF,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager and with no feature within agent config
	e := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{dir},
		PassthroughEnvironment: passthroughEnv,
		// AgentFeatures: map[string]bool{"docker_enabled": false},
	}, config.NewPathLoader(), e, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// AND the manager loads and executes the integrations in the folder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN integration does not emit data
	require.NoError(t, e.ExpectTimeout("nri-docker", 400*time.Millisecond))

	// AND WHEN the OHI is enabled (originally through cmd-channel)
	assert.NoError(t, mgr.EnableOHIFromFF(context.Background(), "docker_enabled"))

	// THEN the integration reports
	metric := expectOneMetric(t, e, "nri-docker")
	require.Equal(t, "hello", metric["value"])
}

func TestManager_EnableFeatureFromAgentConfig(t *testing.T) {
	// GIVEN a configuration file for an OHI with feature in it
	dir, err := tempFiles(map[string]string{
		"foo.yaml": v4FileWithNriDockerNameAndDockerFF,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager and with feature enabled within agent config
	e := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{dir},
		AgentFeatures:          map[string]bool{"docker_enabled": true},
		PassthroughEnvironment: passthroughEnv,
	}, config.NewPathLoader(), e, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// AND the manager starts
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN integration emits data
	metric := expectOneMetric(t, e, "nri-docker")
	require.Equal(t, "hello", metric["value"])
}

func TestManager_CCDisablesAgentEnabledFeature(t *testing.T) {
	skipIfWindows(t)
	// GIVEN a configuration file with a featured OHI that reports continuously.
	dir, err := tempFiles(map[string]string{
		"foo.yaml": v4FileWithContinuousNriDocker,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager and OHI enabled (ie via feature agent config)
	e := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{dir},
		AgentFeatures:          map[string]bool{"docker_enabled": true},
		PassthroughEnvironment: passthroughEnv,
	}, config.NewPathLoader(), e, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// AND manager loads and executes the integrations in the folder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN integration emits data
	metric := expectOneMetric(t, e, "nri-docker")
	require.Equal(t, "first", metric["value"])

	// WHEN the OHI is disabled (originally through cmd-channel)
	assert.NoError(t, mgr.DisableOHIFromFF("docker_enabled"))

	// THEN integration does not report (eventually)
	// the integration may have sent more than 1 event so we have to "drain" the channel
	testhelpers.Eventually(t, 5*time.Second, func(t require.TestingT) {
		require.NoError(t, e.ExpectTimeout("nri-docker", 400*time.Millisecond))
	})
}

func TestManager_CCDisablesPreviouslyEnabledFeature(t *testing.T) {
	skipIfWindows(t)
	// GIVEN a configuration file with a featured OHI that reports continuously.
	dir, err := tempFiles(map[string]string{
		"foo.yaml": v4FileWithContinuousNriDocker,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager and OHI enabled (ie via feature agent config)
	e := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{dir},
		PassthroughEnvironment: passthroughEnv,
	}, config.NewPathLoader(), e, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// AND manager loads and executes the integrations in the folder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN integration does not emit data
	require.NoError(t, e.ExpectTimeout("nri-docker", 400*time.Millisecond))

	// AND WHEN the OHI is enabled (through cmd-channel)
	assert.NoError(t, mgr.EnableOHIFromFF(context.Background(), "docker_enabled"))

	// THEN integration emits data
	metric := expectOneMetric(t, e, "nri-docker")
	require.Equal(t, "first", metric["value"])

	// WHEN the OHI is disabled (originally through cmd-channel)
	assert.NoError(t, mgr.DisableOHIFromFF("docker_enabled"))

	// THEN integration does not report (eventually)
	// the integration may have sent more than 1 event so we have to "drain" the channel
	testhelpers.Eventually(t, 5*time.Second, func(t require.TestingT) {
		require.NoError(t, e.ExpectTimeout("nri-docker", 400*time.Millisecond))
	})
}

func TestManager_WhenFileExists(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "when_file_exists")
	require.NoError(t, err)
	_, err = tmpFile.Write([]byte{})
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	// GIVEN an integration configured with a when: file_exists clause
	// pointing to a file that exists
	dir, err := tempFiles(map[string]string{
		"file.yaml": fmt.Sprintf(v4FileWithWhen, tmpFile.Name()),
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager loads and executes the integrations in the folder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integration runs and emits data
	metric := expectOneMetric(t, emitter, "hello-test")
	assert.Equal(t, "hello", metric["value"])
}

func TestManager_WhenFileDoesNotExist(t *testing.T) {
	// GIVEN an integration configured with a when: file_exists clause
	// pointing to a file that DOES not exist
	dir, err := tempFiles(map[string]string{
		"file.yaml": fmt.Sprintf(v4FileWithWhen, "unexisting_file"),
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager loads and executes the integrations in the folder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integration DOES NOT emit any data
	assert.NoError(t, emitter.ExpectTimeout("hello-test", 500*time.Millisecond))
}

func TestManager_StartWithVerbose(t *testing.T) {
	// GIVEN a configuration file for an OHI with feature in it
	dir, err := tempFiles(map[string]string{
		"foo.yaml": getV4VerboseCheckYAML(t),
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager and with feature enabled within agent config
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{dir},
		PassthroughEnvironment: passthroughEnv,
		Verbose:                1,
	}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// AND the manager starts
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	d := getEmittedData(t, emitter, "verbose-check")
	assert.Contains(t, d.DataSet.Metrics, protocol.MetricData{
		"value":      "true",
		"event_type": "THIS_IS_A_TEST",
	})
	assert.Contains(t, d.DataSet.Metrics, protocol.MetricData{
		"value":      "1",
		"event_type": "VERBOSE",
	})
}

func TestManager_StartWithVerboseFalse(t *testing.T) {
	// GIVEN a configuration file for an OHI with feature in it
	dir, err := tempFiles(map[string]string{
		"foo.yaml": getV4VerboseCheckYAML(t),
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager and with feature enabled within agent config
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:            []string{dir},
		PassthroughEnvironment: passthroughEnv,
		Verbose:                0,
	}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// AND the manager starts
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	d := getEmittedData(t, emitter, "verbose-check")
	assert.Contains(t, d.DataSet.Metrics, protocol.MetricData{
		"value":      "true",
		"event_type": "THIS_IS_A_TEST",
	})
	assert.NotContains(t, d.DataSet.Metrics, protocol.MetricData{
		"value":      "1",
		"event_type": "VERBOSE",
	})
	assert.NotContains(t, d.DataSet.Metrics, protocol.MetricData{
		"value":      "0",
		"event_type": "VERBOSE",
	})
}

func getV4VerboseCheckYAML(t *testing.T) string {
	//      GOTMPDIR: %s
	//      GOCACHE: %s
	goTmp, err := testhelp.GetGoEnv("GOTMPDIR")
	require.NoError(t, err)
	goCache, err := testhelp.GetGoEnv("GOCACHE")
	require.NoError(t, err)
	return fmt.Sprintf(v4VerboseCheck, goTmp, goCache)
}

func TestManager_contextWithVerbose(t *testing.T) {
	actualContext := contextWithVerbose(context.Background(), 1)

	// THEN verbose variable in context set to 1
	assert.Equal(t, actualContext.Value(constants.EnableVerbose), 1)
}

func TestManager_anIntegrationCanSpawnAnotherOne(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no windows support on cmd request for now")
	}

	// GIVEN a configuration file for an integration that will request a cmd request
	dir, err := tempFiles(map[string]string{
		"v4-cmdreq.yaml": v4CmdRequest,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager executes the integration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integration is executed, requesting a new integration run that generates telemetry data
	metric := expectOneMetric(t, emitter, "cmd-req-name")
	assert.Equal(t, "ShellTestSample", metric["event_type"])
}

func TestManager_cfgProtocolSpawnIntegrationV3Payload(t *testing.T) {
	skipIfWindows(t)
	// GIVEN a configuration file for an integration that will send a cfg protocol payload
	dir, err := tempFiles(map[string]string{
		"v4-cfgreq-v3payload.yaml": v4CfgReqV3Payload,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager executes the integration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integration is executed, requesting a new integration run that generates telemetry data of v3 protocol
	metric := expectOneMetric(t, emitter, "spawned_integration")
	assert.Equal(t, "ShellTestSample", metric["event_type"])
}

func TestManager_cfgProtocolSpawnIntegrationV4Payload(t *testing.T) {
	skipIfWindows(t)
	// GIVEN a configuration file for an integration that will send a cfg protocol payload
	dir, err := tempFiles(map[string]string{
		"v4-cfgreq-v4payload.yaml": v4CfgReqV4Payload,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager executes the integration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN the integration is executed, requesting a new integration run that generates telemetry data of v4 protocol
	assert.Len(t, expectNMetrics(t, emitter, "spawned_integration", 1), 1)
}

func TestManager_cfgProtocolSpawnedIntegrationCannotSpawnIntegration(t *testing.T) {
	skipIfWindows(t)
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := test.NewGlobal()

	// GIVEN a configuration file for an integration that will send a cfg protocol payload
	dir, err := tempFiles(map[string]string{
		"v4-cfgreq-recursive.yaml": v4CfgReqRecursive,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{ConfigPaths: []string{dir}, PassthroughEnvironment: passthroughEnv}, config.NewPathLoader(), emitter, integration.ErrLookup, definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})

	// WHEN the manager executes the integration
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	// THEN log entry found
	testhelpers.Eventually(t, time.Second*3, func(t require.TestingT) {
		entries := hook.AllEntries()
		require.NotEmpty(t, entries)
		ok := false
		for _, e := range entries {
			if e.Message == "received config protocol request payload without a handler" {
				ok = true
			}
		}
		assert.True(t, ok, "expected log not received")
	})
}

func TestManager_ExpandsConfigEnvVars(t *testing.T) {
	// GIVEN an set of agent directories containing compiled binaries
	niDir, err := ioutil.TempDir("", "newrelic-integrations")
	require.NoError(t, err)
	defer removeTempFiles(t, niDir)
	execPath := filepath.Join(niDir, "nri-validyaml"+fixtures.CmdExtension)
	require.NoError(t, testhelp.GoBuild(fixtures.ValidYAMLGoFile, execPath))

	// AND a v4 named integration with an embedded config
	configDir, err := tempFiles(map[string]string{
		"my-configs.yml": `
integrations:
  - name: nri-validyaml
    config:
      event_type: YAMLEvent
      map:
        hello: {{ SOME_VAR }}  
`,
	})
	require.NoError(t, err)
	defer removeTempFiles(t, configDir)

	// WHEN env-var is set up
	assert.NoError(t, os.Setenv("SOME_VAR", "FOO"))

	// AND the v4 integrations are run
	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(ManagerConfig{
		ConfigPaths:       []string{configDir},
		DefinitionFolders: []string{niDir},
	}, config.NewPathLoader(), emitter, instancesLookupReturning(execPath), definitionQ, configEntryQ, track.NewTracker(nil), host.IDLookup{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go mgr.Start(ctx)

	assert.NoError(t, os.Unsetenv("SOME_VAR"))

	// THEN manager expanded env-var
	metric := expectOneMetric(t, emitter, "nri-validyaml")
	assert.Equal(t, "YAMLEvent", metric["event_type"])
	gotest.DeepEqual(t, map[string]interface{}{"hello": "FOO"}, metric["map"])
}

func TestManager_RunOnce(t *testing.T) {
	// GIVEN an integration returning a protocol v4 payload
	dir, err := tempFiles(map[string]string{
		"protocol_v4.yml": `
integrations:
  - name: nri-protocol-v4
    exec: ` + getExe(testhelp.GoRun(fixtures.ProtocolV4GoFile)),
	})
	require.NoError(t, err)
	defer removeTempFiles(t, dir)

	// AND an integrations manager

	emitter := &testemit.RecordEmitter{}
	mgr := NewManager(
		ManagerConfig{
			ConfigPaths:            []string{dir},
			PassthroughEnvironment: passthroughEnv,
		},
		config.NewPathLoader(),
		emitter,
		integration.ErrLookup,
		definitionQ,
		configEntryQ,
		track.NewTracker(nil),
		host.IDLookup{},
	)

	// WHEN the manager loads and executes the integration
	ctx, cancel := context.WithCancel(context.Background())

	finish := make(chan struct{})

	go func() {
		mgr.RunOnce(ctx)
		close(finish)
	}()

	// THEN emitted metrics are received (gauge, count & summary)
	_ = expectNMetrics(t, emitter, "nri-protocol-v4", 3)
	cancel()

	<-finish
}

func tempFiles(pathContents map[string]string) (directory string, err error) {
	dir, err := ioutil.TempDir("", "tempFiles")
	if err != nil {
		return "", err
	}
	for path, content := range pathContents {
		if err := ioutil.WriteFile(filepath.Join(dir, path), []byte(content), 0o600); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func fileAppend(filePath, content string) error {
	fh, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer func() { _ = fh.Close() }()
	_, err = fh.WriteString(content)
	return err
}

// this receives the next sample from the plugin, expecting a payload with a single metric and returning it
// if nothing is received or the payload has not
func expectOneMetric(t require.TestingT, e *testemit.RecordEmitter, pluginName string) protocol.MetricData {
	return expectNMetrics(t, e, pluginName, 1)[0]
}

func expectNMetrics(t require.TestingT, e *testemit.RecordEmitter, pluginName string, amount int) []protocol.MetricData {
	dataset := getEmittedData(t, e, pluginName)
	require.Len(t, dataset.DataSet.Metrics, amount)
	return dataset.DataSet.Metrics
}

func getEmittedData(t require.TestingT, e *testemit.RecordEmitter, pluginName string) testemit.EmittedData {
	dataset, err := e.ReceiveFrom(pluginName)
	require.NoError(t, err)
	return dataset
}

func expectNoMetric(t require.TestingT, e *testemit.RecordEmitter, pluginName string) {
	_, err := e.ReceiveFrom(pluginName)
	require.Error(t, err)
}

func skipIfWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping in windows")
	}
}

func instancesLookupReturning(execPaths ...string) integration.InstancesLookup {
	calls := 0
	return integration.InstancesLookup{
		Legacy: func(dcc integration.DefinitionCommandConfig) (integration.Definition, error) {
			return integration.Definition{}, errors.New("legacy lookup not expected")
		},
		ByName: func(_ string) (string, error) {
			il := execPaths[calls]
			// use last provided path when not enough
			if len(execPaths) >= calls+1 {
				calls++
			}
			return il, nil
		},
	}
}

func instancesLookupLegacy(definitionFolders ...string) integration.InstancesLookup {
	legacyDefinedCommands := v3legacy.NewDefinitionsRepo(v3legacy.LegacyConfig{
		DefinitionFolders: definitionFolders,
		Verbose:           0,
	})
	return integration.InstancesLookup{
		Legacy: legacyDefinedCommands.NewDefinitionCommand,
		ByName: func(_ string) (string, error) {
			return "", errors.New("lookup by name not expected")
		},
	}
}
