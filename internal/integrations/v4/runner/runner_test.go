package runner

import (
	"context"
	"io/ioutil"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/cache"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cfgProtocol = `{
    "config_protocol_version": "1",
    "action": "register_config",
    "config_name": "myconfig",
    "config": {
        "variables": {},
        "integrations": [
            {
                "name": "nri-test",
                "exec": [
                    "echo {}"
                ]
            }
        ]
    }
}`

func Test_runner_Run(t *testing.T) {
	def, err := integration.NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.IntegrationScript, "bar"),
	}, integration.ErrLookup, nil, nil)
	require.NoError(t, err)

	e := &testemit.RecordEmitter{}
	r := NewRunner(def, e, nil, nil, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	r.Run(ctx, nil, nil)

	dataset, err := e.ReceiveFrom("foo")
	require.NoError(t, err)
	metrics := dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	assert.Equal(t, "TestSample", metrics[0]["event_type"])
	assert.Equal(t, "bar", metrics[0]["value"])
	assert.Empty(t, dataset.Metadata.Labels)
}

func Test_runner_Run_noHandleForCfgProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	// GIVEN a runner that receives a cfg request without a handle function.
	def, err := integration.NewDefinition(config.ConfigEntry{
		InstanceName: "Parent",
		Exec:         testhelp.Command(fixtures.EchoFromEnv),
		Env:          map[string]string{"STDOUT_STRING": cfgProtocol},
	}, integration.ErrLookup, nil, nil)
	require.NoError(t, err)

	e := &testemit.RecordEmitter{}
	r := NewRunner(def, e, nil, nil, cmdrequest.NoopHandleFn, nil, nil)

	// WHEN the runner executes the binary and handle the payload.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	r.Run(ctx, nil, nil)

	// THEN log entry found.
	assert.Eventually(t, func() bool {
		entries := hook.AllEntries()
		for _, e := range entries {
			if e.Message == "received config protocol request payload without a handler" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	le := hook.LastEntry()
	require.NotNil(t, le)
	assert.Equal(t, logrus.WarnLevel, le.Level)

}
func Test_runner_Run_failToUnMarshallCfgProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	// GIVEN a runner that receives a cfg request without a handle function.
	def, err := integration.NewDefinition(config.ConfigEntry{
		InstanceName: "Parent",
		Exec:         testhelp.Command(fixtures.EchoFromEnv),
		Env: map[string]string{"STDOUT_STRING": `{
			"config_protocol_version": "1",
			"action": "register_config"
		}`},
	}, integration.ErrLookup, nil, nil)
	require.NoError(t, err)

	e := &testemit.RecordEmitter{}
	r := NewRunner(def, e, nil, nil, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, nil)

	// WHEN the runner executes the binary and handle the payload.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	r.Run(ctx, nil, nil)

	// THEN log entry found.
	assert.Eventually(t, func() bool {
		entries := hook.AllEntries()
		for _, e := range entries {
			if e.Message == "cannot build config protocol" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
	le := hook.LastEntry()
	require.NotNil(t, le)
	assert.Equal(t, logrus.WarnLevel, le.Level)

}
func Test_runner_Run_handlesCfgProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	// GIVEN a runner that receives a cfg request.
	def, err := integration.NewDefinition(config.ConfigEntry{
		InstanceName: "Parent",
		Exec:         testhelp.Command(fixtures.EchoFromEnv),
		Env:          map[string]string{"STDOUT_STRING": cfgProtocol},
	}, integration.ErrLookup, nil, nil)
	require.NoError(t, err)

	var called uint32
	mockHandleFn := func(cfgProtocol protocol.ConfigProtocol, c cache.Cache, parentDefinition integration.Definition) {
		atomic.AddUint32(&called, 1)
	}
	e := &testemit.RecordEmitter{}
	r := NewRunner(def, e, nil, nil, cmdrequest.NoopHandleFn, mockHandleFn, nil)

	// WHEN the runner executes the binary and handle the payload.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	r.Run(ctx, nil, nil)

	// THEN the config request is processed by the handle function.
	assert.Eventually(t, func() bool {
		if c := atomic.LoadUint32(&called); c > 0 {
			return true
		}
		return false
	}, time.Second, 10*time.Millisecond)
}
