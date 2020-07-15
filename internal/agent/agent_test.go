// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	context2 "context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags/test"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/ctl"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
)

var NilIDLookup IDLookup

var matcher = func(interface{}) bool { return true }

func newTesting(cfg *config.Config) *Agent {
	dataDir, err := ioutil.TempDir("", "prefix")
	if err != nil {
		panic(err)
	}

	if cfg == nil {
		cfg = config.NewTest(dataDir)
	}
	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	lookups := NewIdLookup(hostname.CreateResolver("", "", true), cloudDetector, cfg.DisplayName)

	//ctx := newContextTesting("1.2.3", cfg)
	ctx := NewContext(cfg, "1.2.3", testhelpers.NullHostnameResolver, lookups, matcher)

	st := delta.NewStore(dataDir, "default", cfg.MaxInventorySize)

	fpHarvester, err := fingerprint.NewHarvestor(cfg, testhelpers.NullHostnameResolver, cloudDetector)
	if err != nil {
		panic(err)
	}

	connectSrv := NewIdentityConnectService(&MockIdentityConnectClient{}, fpHarvester)
	provideIDs := NewProvideIDs(&EmptyRegisterClient{}, state.NewRegisterSM())

	a, err := New(cfg, ctx, "user-agent", lookups, st, connectSrv, provideIDs, http2.NullHttpClient, &http.Transport{}, cloudDetector, fpHarvester, ctl.NewNotificationHandlerWithCancellation(nil))

	if err != nil {
		panic(err)
	}

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
	a := newTesting(&config.Config{
		IgnoredInventoryPathsMap: map[string]struct{}{
			"test/plugin/yum": {},
		},
		MaxInventorySize: 1024,
	})
	defer func() {
		_ = os.RemoveAll(a.store.DataDir)
	}()

	assert.NoError(t, a.storePluginOutput(PluginOutput{
		Id:        ids.PluginID{"test", "plugin"},
		EntityKey: "someEntity",
		Data: PluginInventoryDataset{
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

	ctx := NewContext(&config.Config{}, "", testhelpers.NullHostnameResolver, NilIDLookup, matcher)
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
	a := newTesting(nil)
	defer os.RemoveAll(a.store.DataDir)

	idMap := IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME: "displayName",
		sysinfo.HOST_SOURCE_HOSTNAME:     "hostName",
		sysinfo.HOST_SOURCE_INSTANCE_ID:  "instanceId",
	}

	a.setAgentKey(idMap)
	assert.Equal(t, idMap[sysinfo.HOST_SOURCE_INSTANCE_ID], a.Context.AgentIdentifier())
}

// Test that empty strings in the identity map are properly ignored in favor of non-empty ones
func TestSetAgentKeysInstanceEmptyString(t *testing.T) {
	a := newTesting(nil)
	defer os.RemoveAll(a.store.DataDir)

	keys := IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME: "displayName",
		sysinfo.HOST_SOURCE_HOSTNAME:     "hostName",
		sysinfo.HOST_SOURCE_INSTANCE_ID:  "",
	}

	a.setAgentKey(keys)
	assert.Equal(t, keys[sysinfo.HOST_SOURCE_DISPLAY_NAME], a.Context.AgentIdentifier())
}

func TestSetAgentKeysDisplayNameMatchesHostName(t *testing.T) {
	a := newTesting(nil)
	defer os.RemoveAll(a.store.DataDir)

	keyMap := IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME: "hostName",
		sysinfo.HOST_SOURCE_HOSTNAME:     "hostName",
	}

	a.setAgentKey(keyMap)
	assert.Equal(t, "hostName", a.Context.AgentIdentifier())
}

func TestSetAgentKeysNoValues(t *testing.T) {
	a := newTesting(nil)
	defer os.RemoveAll(a.store.DataDir)

	assert.Error(t, a.setAgentKey(IDLookup{}))
}

func TestUpdateIDLookupTable(t *testing.T) {
	a := newTesting(nil)
	defer os.RemoveAll(a.store.DataDir)

	dataset := PluginInventoryDataset{}
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
	assert.Equal(t, "instanceId", a.Context.AgentIdentifier())
}

func TestIDLookup_EntityNameCloudInstance(t *testing.T) {
	l := IDLookup{
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
	l := IDLookup{
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
	l := IDLookup{
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
	l := IDLookup{
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
	l := IDLookup{
		sysinfo.HOST_SOURCE_DISPLAY_NAME:   "display-name",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}
	name, err := l.AgentShortEntityName()

	assert.NoError(t, err)
	assert.Equal(t, "display-name", name)
}

func TestIDLookup_EntityNameShortName(t *testing.T) {
	l := IDLookup{
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
	agent := newTesting(nil)
	defer os.RemoveAll(agent.store.DataDir)
	agent.inventories = map[string]*inventory{}

	dataDir := agent.store.DataDir

	// With a set of registered entities
	for _, id := range []string{"entity:1", "entity:2", "entity:3"} {
		agent.registerEntityInventory(id)
		assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, helpers.SanitizeFileName(id)), 0755))
		assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, helpers.SanitizeFileName(id)), 0755))
	}
	// With some entity inventory folders from previous executions
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity4"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity5"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, aPlugin, "entity6"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity4"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity5"), 0755))
	assert.NoError(t, os.MkdirAll(filepath.Join(dataDir, anotherPlugin, "entity6"), 0755))

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
	a := newTesting(nil)
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
	a, err := NewAgent(cnf, "testing-timeouts", ffFetcher)
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
	_, err := NewAgent(cnf, "testing-timeouts", ffFetcher)
	assert.Error(t, err)
}

func TestCheckNetworkNoTimeout(t *testing.T) {
	retval := make(chan error, 1)

	// Given a server that always returns timeouts
	ts := NewTimeoutServer(-1)
	defer ts.Cancel()

	go func() {
		cnf := &config.Config{
			CollectorURL:             ts.server.URL,
			StartupConnectionRetries: -1,
			StartupConnectionTimeout: "1ms",
			MaxInventorySize:         maxInventoryDataSize,
		}

		backOff := &backoff.Backoff{Min: 1 * time.Millisecond}
		retrier := backoff.NewRetrierWithBackoff(backOff)
		retval <- checkCollectorConnectivity(context2.Background(), cnf, retrier, "testing-interruption", "agent-key", &http.Transport{})
	}()

	select {
	case err := <-retval:
		// this should never be triggered
		assert.Error(t, err)
		break
	case <-time.After(100 * time.Millisecond):
		break
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
	a := newTesting(nil)
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

	a := newTesting(nil)

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
	a := newTesting(nil)
	defer os.RemoveAll(a.store.DataDir)
	aV := "aValue"
	bV := "bValue"
	cV := "cValue"
	err := a.storePluginOutput(PluginOutput{
		Id:        ids.PluginID{"test", "plugin"},
		EntityKey: "someEntity",
		Data: PluginInventoryDataset{
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
	a := newTesting(&config.Config{MaxInventorySize: 1000 * 1000})
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
			var dataset PluginInventoryDataset
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

			output := PluginOutput{
				Id:        ids.PluginID{"test", "plugin"},
				EntityKey: "someEntity",
				Data:      dataset,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = a.storePluginOutput(output)
			}
			b.StopTimer()
		})
	}
}

func Test_ProcessSampling_FeatureFlagIsEnabled(t *testing.T) {
	cnf := &config.Config{
		IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}},
	}
	someSample := struct {
		evenType string
	}{
		evenType: "ProcessSample",
	}
	a, _ := NewAgent(cnf, "test", test.NewFFRetrieverReturning(true, true))

	// when
	actual := a.Context.shouldIncludeEvent(someSample)

	// then
	assert.Equal(t, true, actual)
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
		a, _ := NewAgent(tc.c, "test", tc.ff)

		t.Run(tc.name, func(t *testing.T) {
			actual := a.Context.shouldIncludeEvent(someSample)
			assert.Equal(t, tc.want, actual)
		})
	}
}
