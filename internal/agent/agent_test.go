// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// nolint:exhaustruct
package agent

import (
	"bytes"
	context2 "context"
	"encoding/json"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	agentTypes "github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags/test"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/ctl"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/metric"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types" //nolint:depguard
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var NilIDLookup host.IDLookup

var matcher = func(interface{}) bool { return true }

func newTesting(cfg *config.Config, ffRetriever feature_flags.Retriever) *Agent {
	dataDir, err := ioutil.TempDir("", "prefix")
	if err != nil {
		panic(err)
	}

	if cfg == nil {
		cfg = config.NewTest(dataDir)
	}
	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	lookups := NewIdLookup(hostname.CreateResolver("", "", true), cloudDetector, cfg.DisplayName)

	ctx := NewContext(cfg, "1.2.3", testhelpers.NullHostnameResolver, lookups, matcher, matcher)

	st := delta.NewStore(dataDir, "default", cfg.MaxInventorySize, true)

	fpHarvester, err := fingerprint.NewHarvestor(cfg, testhelpers.NullHostnameResolver, cloudDetector)
	if err != nil {
		panic(err)
	}

	metadataHarvester := &identityapi.MetadataHarvesterMock{}
	metadataHarvester.ShouldHarvest(identityapi.Metadata{})

	registerClient := &identityapi.RegisterClientMock{}
	connectSrv := NewIdentityConnectService(&MockIdentityConnectClient{}, fpHarvester, metadataHarvester)
	provideIDs := NewProvideIDs(registerClient, state.NewRegisterSM())

	a, err := New(
		cfg,
		ctx,
		"user-agent",
		lookups,
		st,
		connectSrv,
		provideIDs,
		http2.NullHttpClient,
		&http.Transport{},
		cloudDetector,
		fpHarvester,
		ctl.NewNotificationHandlerWithCancellation(nil),
		ffRetriever,
	)
	if err != nil {
		panic(err)
	}

	a.Init()

	return a
}

type TestAgentData struct {
	Name  string
	Value string
}

func (self *TestAgentData) SortKey() string {
	return self.Name
}

func TestIgnoreInventory(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(&config.Config{
		IgnoredInventoryPathsMap: map[string]struct{}{
			"test/plugin/yum": {},
		},
		MaxInventorySize: 1024,
	}, ffRetriever)
	defer func() {
		_ = os.RemoveAll(a.store.DataDir)
	}()

	assert.NoError(t, a.storePluginOutput(agentTypes.PluginOutput{
		Id:     ids.PluginID{"test", "plugin"},
		Entity: entity.NewFromNameWithoutID("someEntity"),
		Data: agentTypes.PluginInventoryDataset{
			&TestAgentData{"yum", "value1"},
			&TestAgentData{"myService", "value2"},
		},
	}))

	restoredDataBytes, err := ioutil.ReadFile(filepath.Join(a.store.DataDir, "test", "someEntity", "plugin.json"))
	require.NoError(t, err)

	var restoredData map[string]interface{}
	require.NoError(t, json.Unmarshal(restoredDataBytes, &restoredData))

	assert.Equal(t, restoredData, map[string]interface{}{
		"myService": map[string]interface{}{
			"Name":  "myService",
			"Value": "value2",
		},
	})
}

func TestServicePidMap(t *testing.T) {
	ctx := NewContext(&config.Config{}, "", testhelpers.NullHostnameResolver, NilIDLookup, matcher, matcher)
	svc, ok := ctx.GetServiceForPid(1)
	assert.False(t, ok)
	assert.Len(t, svc, 0)

	ctx.CacheServicePids(sysinfo.PROCESS_NAME_SOURCE_SYSTEMD, map[int]string{1: "abc", 2: "def"})
	ctx.CacheServicePids(sysinfo.PROCESS_NAME_SOURCE_SYSVINIT, map[int]string{1: "abc-sysv", 2: "def-sysv"})

	svc, ok = ctx.GetServiceForPid(1)
	assert.True(t, ok)
	assert.Equal(t, "abc", svc)
}

func TestSetAgentKeysDisplayInstance(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)

	idMap := host.IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME: "displayName",
		sysinfo.HOST_SOURCE_HOSTNAME:     "hostName",
		sysinfo.HOST_SOURCE_INSTANCE_ID:  "instanceId",
	}

	a.setAgentKey(idMap)
	assert.Equal(t, idMap[sysinfo.HOST_SOURCE_INSTANCE_ID], a.Context.EntityKey())
}

// Test that empty strings in the identity map are properly ignored in favor of non-empty ones
func TestSetAgentKeysInstanceEmptyString(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)

	keys := host.IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME: "displayName",
		sysinfo.HOST_SOURCE_HOSTNAME:     "hostName",
		sysinfo.HOST_SOURCE_INSTANCE_ID:  "",
	}

	a.setAgentKey(keys)
	assert.Equal(t, keys[sysinfo.HOST_SOURCE_DISPLAY_NAME], a.Context.EntityKey())
}

func TestSetAgentKeysDisplayNameMatchesHostName(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)

	keyMap := host.IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME: "hostName",
		sysinfo.HOST_SOURCE_HOSTNAME:     "hostName",
	}

	a.setAgentKey(keyMap)
	assert.Equal(t, "hostName", a.Context.EntityKey())
}

func TestSetAgentKeysNoValues(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)

	assert.Error(t, a.setAgentKey(host.IDLookup{}))
}

func TestUpdateIDLookupTable(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)

	dataset := agentTypes.PluginInventoryDataset{}
	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  "hostName.com",
		Source: sysinfo.HOST_SOURCE_HOSTNAME,
	})
	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  "instanceId",
		Source: sysinfo.HOST_SOURCE_INSTANCE_ID,
	})
	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  "hostName",
		Source: sysinfo.HOST_SOURCE_HOSTNAME_SHORT,
	})

	assert.NoError(t, a.updateIDLookupTable(dataset))
	assert.Equal(t, "instanceId", a.Context.EntityKey())
}

func TestIDLookup_EntityNameCloudInstance(t *testing.T) {
	l := host.IDLookup{
		sysinfo.HOST_SOURCE_INSTANCE_ID:    "instance-id",
		sysinfo.HOST_SOURCE_AZURE_VM_ID:    "azure-id",
		sysinfo.HOST_SOURCE_GCP_VM_ID:      "gcp-id",
		sysinfo.HOST_SOURCE_ALIBABA_VM_ID:  "alibaba-id",
		sysinfo.HOST_SOURCE_DISPLAY_NAME:   "display-name",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}

	name, err := l.AgentShortEntityName()

	assert.NoError(t, err)
	assert.Equal(t, "instance-id", name)
}

func TestIDLookup_EntityNameAzure(t *testing.T) {
	l := host.IDLookup{
		sysinfo.HOST_SOURCE_INSTANCE_ID:    "",
		sysinfo.HOST_SOURCE_AZURE_VM_ID:    "azure-id",
		sysinfo.HOST_SOURCE_GCP_VM_ID:      "gcp-id",
		sysinfo.HOST_SOURCE_ALIBABA_VM_ID:  "alibaba-id",
		sysinfo.HOST_SOURCE_DISPLAY_NAME:   "display-name",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}
	name, err := l.AgentShortEntityName()

	assert.NoError(t, err)
	assert.Equal(t, "azure-id", name)
}

func TestIDLookup_EntityNameGCP(t *testing.T) {
	l := host.IDLookup{
		sysinfo.HOST_SOURCE_INSTANCE_ID:    "",
		sysinfo.HOST_SOURCE_AZURE_VM_ID:    "",
		sysinfo.HOST_SOURCE_GCP_VM_ID:      "gcp-id",
		sysinfo.HOST_SOURCE_ALIBABA_VM_ID:  "alibaba-id",
		sysinfo.HOST_SOURCE_DISPLAY_NAME:   "display-name",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}
	name, err := l.AgentShortEntityName()

	assert.NoError(t, err)
	assert.Equal(t, "gcp-id", name)
}

func TestIDLookup_EntityNameAlibaba(t *testing.T) {
	l := host.IDLookup{
		sysinfo.HOST_SOURCE_INSTANCE_ID:    "",
		sysinfo.HOST_SOURCE_AZURE_VM_ID:    "",
		sysinfo.HOST_SOURCE_GCP_VM_ID:      "",
		sysinfo.HOST_SOURCE_ALIBABA_VM_ID:  "alibaba-id",
		sysinfo.HOST_SOURCE_DISPLAY_NAME:   "display-name",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}
	name, err := l.AgentShortEntityName()

	assert.NoError(t, err)
	assert.Equal(t, "alibaba-id", name)
}

func TestIDLookup_EntityNameDisplayName(t *testing.T) {
	l := host.IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME:   "display-name",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}
	name, err := l.AgentShortEntityName()

	assert.NoError(t, err)
	assert.Equal(t, "display-name", name)
}

func TestIDLookup_EntityNameShortName(t *testing.T) {
	l := host.IDLookup{
		sysinfo.HOST_SOURCE_HOSTNAME:       "long",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}
	name, err := l.AgentShortEntityName()

	assert.NoError(t, err)
	assert.Equal(t, "short", name)
}

func TestRemoveOutdatedEntities(t *testing.T) {
	const aPlugin = "aPlugin"
	const anotherPlugin = "anotherPlugin"

	// Given an agent
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	agent := newTesting(nil, ffRetriever)
	defer os.RemoveAll(agent.store.DataDir)
	agent.inventories = map[string]*inventoryEntity{}

	dataDir := agent.store.DataDir

	// With a set of registered entities
	for _, id := range []string{"entity:1", "entity:2", "entity:3"} {
		agent.registerEntityInventory(entity.NewFromNameWithoutID(id))
		require.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, helpers.SanitizeFileName(id)), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, helpers.SanitizeFileName(id)), 0o755))
	}
	// With some entity inventory folders from previous executions
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity4"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity5"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity6"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity4"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity5"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity6"), 0o755))

	// When not all the entities reported information during the last period
	entitiesThatReported := map[string]bool{
		"entity:2": true,
	}
	// And the "remove outdated entities" is triggered
	agent.removeOutdatedEntities(entitiesThatReported)

	// Then the entities that didn't reported information have been unregistered
	// and only their folders are kept
	entities := []struct {
		ID                 string
		Folder             string
		ShouldBeRegistered bool
	}{
		{"entity:1", "entity1", false},
		{"entity:2", "entity2", true},
		{"entity:3", "entity3", false},
		{"dontCare", "entity4", false},
		{"doesntMatter", "entity5", false},
	}
	for _, entity := range entities {
		_, ok := agent.inventories[entity.ID]
		assert.Equal(t, entity.ShouldBeRegistered, ok)

		_, err1 := os.Stat(filepath.Join(dataDir, aPlugin, entity.Folder))
		_, err2 := os.Stat(filepath.Join(dataDir, anotherPlugin, entity.Folder))
		if entity.ShouldBeRegistered {
			assert.NoError(t, err1)
			assert.NoError(t, err2)
		} else {
			assert.True(t, os.IsNotExist(err1))
			assert.True(t, os.IsNotExist(err2))
		}
	}
}

func TestReconnectablePlugins(t *testing.T) {
	// Given an agent
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)

	wg := sync.WaitGroup{}
	wg.Add(2)
	// With a set of registered plugins
	nrp := nonReconnectingPlugin{invocations: 0, wg: &wg}
	a.RegisterPlugin(&nrp)
	rp := reconnectingPlugin{invocations: 0, context: a.Context, wg: &wg}
	a.RegisterPlugin(&rp)

	// That successfully started
	a.startPlugins()
	assert.NoError(t, wait(time.Second, &wg))

	// When the agent reconnects
	wg.Add(1)
	a.Context.Reconnect()
	assert.NoError(t, wait(time.Second, &wg))

	// The non-reconnecting plugins are not invoked again
	assert.Equal(t, 1, nrp.invocations)
	// And the reconnecting plugins are invoked again
	assert.Equal(t, 2, rp.invocations)
}

func TestCheckConnectionRetry(t *testing.T) {
	// Given a server that returns timeouts and eventually accepts the requests
	ts := NewTimeoutServer(2)
	defer ts.Cancel()

	cnf := &config.Config{
		CollectorURL:             ts.server.URL,
		StartupConnectionRetries: 3,
		StartupConnectionTimeout: "10ms",
		MaxInventorySize:         maxInventoryDataSize,
	}

	// required for building the agent
	ffFetcher := test.NewFFRetrieverReturning(false, false)

	// The agent should eventually connect
	a, err := NewAgent(cnf, "testing-timeouts", "userAgent", ffFetcher)
	assert.NoError(t, err)
	assert.NotNil(t, a)
}

func TestCheckConnectionTimeout(t *testing.T) {
	// Given a server that always returns timeouts
	ts := NewTimeoutServer(3)
	defer ts.Cancel()

	cnf := &config.Config{
		CollectorURL:             ts.server.URL,
		StartupConnectionRetries: 2,
		StartupConnectionTimeout: "1ms",
		MaxInventorySize:         maxInventoryDataSize,
	}

	// required to build the agent
	ffFetcher := test.NewFFRetrieverReturning(false, false)

	// The agent stops reconnecting after retrying as configured
	_, err := NewAgent(cnf, "testing-timeouts", "userAgent", ffFetcher)
	assert.Error(t, err)
}

func Test_checkCollectorConnectivity_NoTimeoutOnInfiniteRetries(t *testing.T) {
	// Given a server that always returns timeouts
	ts := NewTimeoutServer(-1)
	defer ts.Cancel()

	// When a connectivity is checked with retries
	connErr := make(chan error, 1)
	go func() {
		cnf := &config.Config{
			CollectorURL:             ts.server.URL,
			StartupConnectionRetries: -1,
			StartupConnectionTimeout: "1ms",
		}

		backOff := &backoff.Backoff{Min: 1 * time.Millisecond}
		retrier := backoff.NewRetrierWithBackoff(backOff)
		connErr <- checkCollectorConnectivity(context2.Background(), cnf, retrier, "testing-interruption", "agent-key", &http.Transport{})
	}()

	// Then no timeout error is returned
	select {
	case err := <-connErr:
		assert.Error(t, err)
		// this should never be triggered
		t.Fail()
	case <-time.After(100 * time.Millisecond):
		// retries keep going on as expected
	}
}

type killingPlugin struct {
	killed bool
}

func (killingPlugin) Run()                          {}
func (killingPlugin) LogInfo()                      {}
func (killingPlugin) ScheduleHealthCheck()          {}
func (p *killingPlugin) Kill()                      { p.killed = true }
func (killingPlugin) Id() ids.PluginID              { return ids.PluginID{} }
func (killingPlugin) IsExternal() bool              { return false }
func (killingPlugin) GetExternalPluginName() string { return "" }

func TestTerminate(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer func() {
		_ = os.RemoveAll(a.store.DataDir)
	}()
	a.plugins = []Plugin{
		&killingPlugin{killed: false}, &killingPlugin{killed: false}, &killingPlugin{killed: false},
	}

	a.Terminate()
	assert.Len(t, a.plugins, 3)
	for _, plugin := range a.plugins {
		assert.True(t, plugin.(*killingPlugin).killed)
	}
}

func TestStopByCancelFn_UsedBySignalHandler(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.ShouldGetFeatureFlag(fflag.FlagFullInventoryDeletion, false, false)
	a := newTesting(nil, ffRetriever)

	defer func() {
		_ = os.RemoveAll(a.store.DataDir)
	}()
	a.plugins = []Plugin{
		&killingPlugin{killed: false}, &killingPlugin{killed: false}, &killingPlugin{killed: false},
	}

	go func() {
		assert.NoError(t, a.Run())
		wg.Done()
	}()

	a.Context.CancelFn()
	wg.Wait()
}

// patchSenderCallRecorder patchSender implementation for tests. It will record the calls made to Process()
type patchSenderCallRecorder struct {
	sync.Mutex
	calls int
}

func (p *patchSenderCallRecorder) Process() error {
	p.Lock()
	defer p.Unlock()
	p.calls++
	return nil
}

func (p *patchSenderCallRecorder) getCalls() int {
	p.Lock()
	p.Unlock()
	return p.calls
}

func TestAgent_Run_DontSendInventoryIfFwdOnly(t *testing.T) {
	tests := []struct {
		name              string
		fwdOnly           bool
		assertFunc        func(t assert.TestingT, e1 interface{}, e2 interface{}, msgAndArgs ...interface{}) bool
		expected          int
		firstReapInterval time.Duration
		sendInterval      time.Duration
	}{
		{
			name:              "forward only enabled should not send inventory",
			fwdOnly:           true,
			assertFunc:        assert.Equal,
			expected:          0,
			firstReapInterval: time.Second,
			sendInterval:      time.Microsecond * 5,
		},
		{
			name:              "forward only enabled should not send inventory low values timers",
			fwdOnly:           true,
			assertFunc:        assert.Equal,
			expected:          0,
			firstReapInterval: time.Nanosecond,
			sendInterval:      time.Nanosecond,
		},
		{
			name:              "forward only disabled should send inventory at least once",
			fwdOnly:           false,
			assertFunc:        assert.GreaterOrEqual,
			expected:          1,
			firstReapInterval: time.Second,
			sendInterval:      time.Microsecond * 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(1)
			cfg := &config.Config{
				IsForwardOnly:     tt.fwdOnly,
				FirstReapInterval: tt.firstReapInterval,
				SendInterval:      tt.sendInterval,
			}

			ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
			ffRetriever.ShouldGetFeatureFlag(fflag.FlagFullInventoryDeletion, false, false)
			a := newTesting(cfg, ffRetriever)
			// Give time to at least send one request
			ctxTimeout, _ := context2.WithTimeout(a.Context.Ctx, time.Millisecond*10)
			a.Context.Ctx = ctxTimeout

			// Inventory recording calls
			snd := &patchSenderCallRecorder{}
			a.inventories = map[string]*inventoryEntity{"test": {sender: snd}}

			go func() {
				assert.NoError(t, a.Run())
				wg.Done()
			}()
			wg.Wait()

			tt.assertFunc(t, snd.getCalls(), tt.expected)
		})
	}
}

func wait(timeout time.Duration, wg *sync.WaitGroup) error {
	done := make(chan bool, 0)
	go func() {
		wg.Wait()
		done <- true
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for task to complete")
	}
}

type reconnectingPlugin struct {
	invocations int
	context     AgentContext
	wg          *sync.WaitGroup
}

func (p *reconnectingPlugin) Run() {
	p.invocations++
	p.context.AddReconnecting(p)
	p.wg.Done()
}

func (reconnectingPlugin) Id() ids.PluginID {
	return ids.PluginID{
		Category: "reconnecting",
		Term:     "plugin",
	}
}

func (reconnectingPlugin) LogInfo() {}

func (reconnectingPlugin) ScheduleHealthCheck() {}

func (reconnectingPlugin) IsExternal() bool {
	return false
}

func (reconnectingPlugin) GetExternalPluginName() string {
	return ""
}

type nonReconnectingPlugin struct {
	invocations int
	wg          *sync.WaitGroup
}

func (p *nonReconnectingPlugin) Run() {
	p.invocations++
	p.wg.Done()
}

func (nonReconnectingPlugin) Id() ids.PluginID {
	return ids.PluginID{
		Category: "non-reconnecting",
		Term:     "plugin",
	}
}

func (nonReconnectingPlugin) LogInfo() {}

func (nonReconnectingPlugin) ScheduleHealthCheck() {}

func (nonReconnectingPlugin) IsExternal() bool {
	return false
}

func (nonReconnectingPlugin) GetExternalPluginName() string {
	return ""
}

type TimeoutServer struct {
	unblock        chan interface{}
	invocations    *int32
	timeoutsNumber int32
	server         *httptest.Server
}

func NewTimeoutServer(timeoutsNumber int32) *TimeoutServer {
	ts := &TimeoutServer{
		unblock:        make(chan interface{}),
		invocations:    new(int32),
		timeoutsNumber: timeoutsNumber,
	}
	ts.server = httptest.NewServer(http.HandlerFunc(ts.handler))
	return ts
}

func (t *TimeoutServer) handler(w http.ResponseWriter, r *http.Request) {
	currentInvocations := atomic.AddInt32(t.invocations, 1)
	if currentInvocations < t.timeoutsNumber || t.timeoutsNumber < 0 {
		<-t.unblock
	}
}

func (t *TimeoutServer) Cancel() {
	close(t.unblock)
}

type testAgentNullableData struct {
	Name  string
	Value *string
}

func (self *testAgentNullableData) SortKey() string {
	return self.Name
}

func TestStorePluginOutput(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(nil, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)
	aV := "aValue"
	bV := "bValue"
	cV := "cValue"
	err := a.storePluginOutput(agentTypes.PluginOutput{
		Id:     ids.PluginID{"test", "plugin"},
		Entity: entity.NewFromNameWithoutID("someEntity"),
		Data: agentTypes.PluginInventoryDataset{
			&testAgentNullableData{"cMyService", &cV},
			&testAgentNullableData{"aMyService", &aV},
			&testAgentNullableData{"NilService", nil},
			&testAgentNullableData{"bMyService", &bV},
		},
	})

	assert.NoError(t, err)

	sourceFile := filepath.Join(a.store.DataDir, "test", "someEntity", "plugin.json")
	sourceB, err := ioutil.ReadFile(sourceFile)
	require.NoError(t, err)

	expected := []byte(`{"NilService":{"Name":"NilService"},"aMyService":{"Name":"aMyService","Value":"aValue"},"bMyService":{"Name":"bMyService","Value":"bValue"},"cMyService":{"Name":"cMyService","Value":"cValue"}}`)

	assert.Equal(t, string(expected), string(sourceB))
}

type mockHostinfoData struct {
	System          string  `json:"id"`
	Distro          *string `json:"distro"`
	KernelVersion   string  `json:"kernel_version"`
	HostType        string  `json:"host_type"`
	CpuName         string  `json:"cpu_name"`
	CpuNum          string  `json:"cpu_num"`
	TotalCpu        string  `json:"total_cpu"`
	Ram             string  `json:"ram"`
	UpSince         string  `json:"boot_timestamp"`
	AgentVersion    string  `json:"agent_version"`
	AgentName       string  `json:"agent_name"`
	AgentMode       string  `json:"agent_mode"`
	OperatingSystem string  `json:"operating_system"`
	ProductUuid     string  `json:"product_uuid"`
	BootId          string  `json:"boot_id"`
}

func (self mockHostinfoData) SortKey() string {
	return self.System
}

func BenchmarkStorePluginOutput(b *testing.B) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	a := newTesting(&config.Config{MaxInventorySize: 1000 * 1000}, ffRetriever)
	defer os.RemoveAll(a.store.DataDir)

	distroName := "Fedora 29 (Cloud Edition)"
	benchmarks := []struct {
		name   string
		distro *string
	}{
		{"with nulls", nil},
		{"without nulls", &distroName},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var dataset agentTypes.PluginInventoryDataset
			for i := 0; i < 6; i++ {
				mHostInfo := &mockHostinfoData{
					System:          fmt.Sprintf("system-%v", i),
					Distro:          bm.distro,
					KernelVersion:   "4.19.9-300.fc29.x86_64",
					HostType:        "innotek GmbH VirtualBox",
					CpuName:         "Intel(R) Core(TM) i7-7700HQ CPU @ 2.80GHz",
					CpuNum:          "4",
					TotalCpu:        "4",
					Ram:             "4036720 kB",
					UpSince:         "2018-12-24 08:18:51",
					AgentVersion:    "1.1.14",
					AgentName:       "Infrastructure",
					OperatingSystem: "linux",
					ProductUuid:     "unknown",
					BootId:          "42b8448d-1c8e-4b8a-b9d1-0400f12c5a29",
				}
				dataset = append(dataset, mHostInfo)
			}

			output := agentTypes.PluginOutput{
				Id:     ids.PluginID{"test", "plugin"},
				Entity: entity.NewFromNameWithoutID("someEntity"),
				Data:   dataset,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = a.storePluginOutput(output)
			}
			b.StopTimer()
		})
	}
}

func getBooleanPtr(val bool) *bool {
	return &val
}

func Test_ProcessSampling(t *testing.T) {
	someSample := &types.ProcessSample{
		ProcessDisplayName: "some-process",
	}

	type testCase struct {
		name string
		c    *config.Config
		ff   feature_flags.Retriever
		want bool
	}
	testCases := []testCase{
		{
			name: "ConfigurationOptionIsDisabled",
			c:    &config.Config{EnableProcessMetrics: getBooleanPtr(false), DisableCloudMetadata: true},
			want: false,
		},
		{
			name: "ConfigurationOptionIsEnabled",
			c:    &config.Config{EnableProcessMetrics: getBooleanPtr(true), DisableCloudMetadata: true},
			want: true,
		},
		{
			// if the matchers are empty (corner case), the FF retriever is checked so it needs to not be nil
			name: "ConfigurationOptionIsNotPresentAndMatchersAreEmptyAndFeatureFlagIsNotConfigured",
			c:    &config.Config{IncludeMetricsMatchers: map[string][]string{}, DisableCloudMetadata: true},
			ff:   test.NewFFRetrieverReturning(false, false),
			want: false,
		},
		{
			name: "ConfigurationOptionIsNotPresentAndMatchersConfiguredDoNotMatch",
			c:    &config.Config{IncludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}}, DisableCloudMetadata: true},
			want: false,
		},
		{
			name: "ConfigurationOptionIsNotPresentAndMatchersConfiguredDoMatch",
			c:    &config.Config{IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, DisableCloudMetadata: true},
			want: true,
		},
		{
			name: "ConfigurationOptionIsNotPresentAndMatchersAreNotConfiguredAndFeatureFlagIsEnabled",
			c:    &config.Config{DisableCloudMetadata: true},
			ff:   test.NewFFRetrieverReturning(true, true),
			want: true,
		},
		{
			name: "ConfigurationOptionIsNotPresentAndMatchersAreNotConfiguredAndFeatureFlagIsDisabled",
			c:    &config.Config{DisableCloudMetadata: true},
			ff:   test.NewFFRetrieverReturning(false, true),
			want: false,
		},
		{
			name: "ConfigurationOptionIsNotPresentAndMatchersAreNotConfiguredAndFeatureFlagIsNotFound",
			c:    &config.Config{DisableCloudMetadata: true},
			ff:   test.NewFFRetrieverReturning(false, false),
			want: false,
		},
		{
			name: "DefaultBehaviourExcludesProcessSamples",
			c:    &config.Config{DisableCloudMetadata: true},
			ff:   test.NewFFRetrieverReturning(false, false),
			want: false,
		},
	}

	for _, tc := range testCases {
		a, _ := NewAgent(tc.c, "test", "userAgent", tc.ff)

		t.Run(tc.name, func(t *testing.T) {
			actual := a.Context.shouldIncludeEvent(someSample)
			assert.Equal(t, tc.want, actual)
		})
	}
}

func Test_ProcessSamplingExcludesAllCases(t *testing.T) {
	t.Parallel()

	someSample := &types.ProcessSample{
		ProcessDisplayName: "some-process",
	}

	boolAsPointer := func(val bool) *bool {
		return &val
	}

	type testCase struct {
		name                   string
		processMetricsEnabled  *bool
		IncludeMetricsMatchers map[string][]string
		ExcludeMetricsMatchers map[string][]string
		expectInclude          bool
	}

	//nolint:dupword
	// Test cases
	// EnableProcessMetrics | IncludeMetricsMatchers | ExcludeMetricsMatchers | Expected include
	// nil | nil | nil | false
	// nil | [process that matches ] | nil | true
	// nil | [process that not matches ] | nil | false
	// nil | [process that matches ] | [process that matches ] | true
	// nil | [process that not matches ] | [process that not matche ] | false
	// nil | nil | [process that matches ] | false
	// nil | nil | [process that not matches ] | false
	// false | nil | nil | false
	// false | [process that matches ] | nil | false
	// false | nil |  [process that not matche ] | false
	// false | [process that matches ] | [process that matches ] | false
	// false | [process that not matches ] | [process that not matche ] | false
	// true | nil | nil | true
	// true | [process that matches ] | nil | true
	// true | [process that not matches ] | nil | false
	// true | [process that matches ] | [process that matches ] | true
	// true | [process that not matches ] | [process that not matche ] | false
	// true | nil  | [process that matches ] | false
	// true | nil  | [process that not matches ] | true

	testCases := []testCase{
		{
			name:                   "nil | nil | nil | false",
			processMetricsEnabled:  nil,
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: nil,
			expectInclude:          false,
		},
		{
			name:                   "nil | [process that matches] | nil | true",
			processMetricsEnabled:  nil,
			IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			ExcludeMetricsMatchers: nil,
			expectInclude:          true,
		},
		{
			name:                   "nil | [process that not matches] | nil | false",
			processMetricsEnabled:  nil,
			IncludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			ExcludeMetricsMatchers: nil,
			expectInclude:          false,
		},
		{
			name:                   "nil | [process that matches] | [process that matches] | true",
			processMetricsEnabled:  nil,
			IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			expectInclude:          true,
		},
		{
			name:                   "nil | [process that not matches] | [process that not matches] | false",
			processMetricsEnabled:  nil,
			IncludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			expectInclude:          false,
		},
		{
			name:                   "nil | nil | [process that matches] | false",
			processMetricsEnabled:  nil,
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			expectInclude:          false,
		},
		{
			name:                   "nil | nil | [process that not matches] | false",
			processMetricsEnabled:  nil,
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			expectInclude:          false,
		},
		{
			name:                   "false | nil | nil | false",
			processMetricsEnabled:  boolAsPointer(false),
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: nil,
			expectInclude:          false,
		},
		{
			name:                   "false | [process that matches] | nil | false",
			processMetricsEnabled:  boolAsPointer(false),
			IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			ExcludeMetricsMatchers: nil,
			expectInclude:          false,
		},
		{
			name:                   "false | nil | [process that not matches] | false",
			processMetricsEnabled:  boolAsPointer(false),
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			expectInclude:          false,
		},
		{
			name:                   "false | [process that matches] | [process that matches] | false",
			processMetricsEnabled:  boolAsPointer(false),
			IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			expectInclude:          false,
		},
		{
			name:                   "false | [process that not matches] | [process that not matches] | false",
			processMetricsEnabled:  boolAsPointer(false),
			IncludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			expectInclude:          false,
		},
		{
			name:                   "true | nil | nil | true",
			processMetricsEnabled:  boolAsPointer(true),
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: nil,
			expectInclude:          true,
		},
		{
			name:                   "true | [process that matches] | nil | true",
			processMetricsEnabled:  boolAsPointer(true),
			IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			ExcludeMetricsMatchers: nil,
			expectInclude:          true,
		},
		{
			name:                   "true | [process that not matches] | nil | false",
			processMetricsEnabled:  boolAsPointer(true),
			IncludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			ExcludeMetricsMatchers: nil,
			expectInclude:          false,
		},
		{
			name:                   "true | [process that matches] | [process that matches] | true",
			processMetricsEnabled:  boolAsPointer(true),
			IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			expectInclude:          true,
		},
		{
			name:                   "true | [process that not matches] | [process that not matches] | false",
			processMetricsEnabled:  boolAsPointer(true),
			IncludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			expectInclude:          false,
		},
		{
			name:                   "true | nil | [process that matches] | false",
			processMetricsEnabled:  boolAsPointer(true),
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
			expectInclude:          false,
		},
		{
			name:                   "true | nil | [process that not matches] | true",
			processMetricsEnabled:  boolAsPointer(true),
			IncludeMetricsMatchers: nil,
			ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}},
			expectInclude:          true,
		},
	}

	for _, tc := range testCases {
		testCase := tc
		cnf := &config.Config{
			EnableProcessMetrics:   testCase.processMetricsEnabled,
			IncludeMetricsMatchers: testCase.IncludeMetricsMatchers,
			ExcludeMetricsMatchers: testCase.ExcludeMetricsMatchers,
			DisableCloudMetadata:   true,
		}

		ff := test.NewFFRetrieverReturning(false, false)
		a, _ := NewAgent(cnf, "test", "userAgent", ff)

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expectInclude, a.Context.IncludeEvent(someSample))
		})
	}
}

type fakeEventSender struct{}

func (f fakeEventSender) QueueEvent(_ sample.Event, _ entity.Key) error {
	return nil
}

func (f fakeEventSender) Start() error {
	panic("implement me")
}

func (f fakeEventSender) Stop() error {
	panic("implement me")
}

func TestContext_SendEvent_LogTruncatedEvent(t *testing.T) {
	// Capture the logs
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetLevel(logrus.DebugLevel)

	cfg := config.Config{TruncTextValues: true}
	c := NewContext(
		&cfg,
		"0.0.0",
		testhelpers.NewFakeHostnameResolver("foobar", "foo", nil),
		NilIDLookup,
		matcher,
		matcher,
	)
	c.eventSender = fakeEventSender{}

	original := strings.Repeat("a", metric.NRDBLimit*2)
	truncated := original[:metric.NRDBLimit]
	ed := map[string]interface{}{"key": original}
	c.SendEvent(mapEvent(ed), "some key")

	written := output.String()
	assert.Contains(t, written, fmt.Sprintf("original=\"+map[key:%s]", original))
	assert.Contains(t, written, fmt.Sprintf("truncated=\"+map[key:%s]", truncated))
}

func TestRunsWithCloudProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cloudProvider string
		assertFunc    func(assert.TestingT, error, ...any) bool
		retries       int
	}{
		{
			name:          "Valid cloud tries (and fails) to get metadata",
			cloudProvider: "aws",
			assertFunc:    assert.Error,
			retries:       0,
		},
		{
			name:          "Valid cloud tries (and fails) to get metadata after 3 retries",
			cloudProvider: "aws",
			assertFunc:    assert.Error,
			retries:       3,
		},
	}

	for _, tt := range tests {
		testCase := tt

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
			//nolint:exhaustruct
			agt := newTesting(&config.Config{
				CloudProvider:      testCase.cloudProvider,
				CloudMaxRetryCount: testCase.retries,
			}, ffRetriever)

			err := agt.Run()

			testCase.assertFunc(t, err)
		})
	}
}

func TestAgent_checkInstanceIDRetry(t *testing.T) {
	t.Parallel()

	type args struct {
		maxRetries  int
		backoffTime int
	}
	tests := []struct {
		name           string
		cloudHarvester cloud.Harvester
		args           args
		wantErr        bool
	}{
		// This will follow the same strategy as the tests for cloud (see cloud_test.go)
		{
			name:           "Test with valid cloudHarvester",
			cloudHarvester: NewMockHarvester(t, cloud.TypeAWS, false),
			args:           args{maxRetries: 3, backoffTime: 2},
			wantErr:        false,
		},
		{
			name:           "Test valid cloudHarvester with 0 retries",
			cloudHarvester: NewMockHarvester(t, cloud.TypeAzure, true),
			args:           args{maxRetries: 0, backoffTime: 1},
			wantErr:        false,
		},
		{
			name:           "Test with valid cloudHarvester (exhaust retries)",
			cloudHarvester: NewMockHarvester(t, cloud.TypeGCP, false),
			args:           args{maxRetries: 1, backoffTime: 1},
			wantErr:        true,
		},
		{
			name:           "Test with invalid cloudHarvester",
			cloudHarvester: NewMockHarvester(t, cloud.TypeNoCloud, false),
			args:           args{maxRetries: 4, backoffTime: 1},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
			a := newTesting(nil, ffRetriever)
			a.cloudHarvester = testCase.cloudHarvester

			if err := a.checkInstanceIDRetry(testCase.args.maxRetries, testCase.args.backoffTime); (err != nil) != testCase.wantErr {
				t.Errorf("Agent.checkInstanceIDRetry() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}
