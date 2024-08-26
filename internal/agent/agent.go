// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	context2 "context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostid"

	"github.com/newrelic/infrastructure-agent/internal/agent/instrumentation"
	"github.com/newrelic/infrastructure-agent/internal/agent/inventory"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/metric"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/ctl"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"

	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/state"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/fingerprint"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"

	"github.com/newrelic/infrastructure-agent/internal/agent/debug"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/agent/id"
	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

const (
	defaultRemoveEntitiesPeriod     = 48 * time.Hour
	activeEntitiesBufferLength      = 32
	defaultBulkInventoryQueueLength = 1000
)

type registerableSender interface {
	Start() error
	Stop() error
}

type inventoryEntity struct {
	reaper       *PatchReaper
	sender       inventory.PatchSender
	needsReaping bool
	needsCleanup bool
}

type Agent struct {
	inv                 inventoryState
	plugins             []Plugin                    // Slice of registered plugins
	oldPlugins          []ids.PluginID              // Deprecated plugins whose cached data must be removed, if existing
	agentDir            string                      // Base data directory for the agent
	extDir              string                      // Location of external data input
	userAgent           string                      // User-Agent making requests to warlock
	inventories         map[string]*inventoryEntity // Inventory reaper and sender instances (key: entity ID)
	Context             *context                    // Agent context data that is passed around the place
	metricsSender       registerableSender
	inventoryHandler    *inventory.Handler
	store               *delta.Store
	debugProvide        debug.Provide
	httpClient          backendhttp.Client // http client for both data submission types: events and inventory
	connectSrv          *identityConnectService
	provideIDs          ProvideIDs
	entityMap           entity.KnownIDs
	fpHarvester         fingerprint.Harvester
	cloudHarvester      cloud.Harvester                          // If it's the case returns information about the cloud where instance is running.
	agentID             *entity.ID                               // pointer as it's referred from several points
	mtx                 sync.Mutex                               // Protect plugins
	notificationHandler *ctl.NotificationHandlerWithCancellation // Handle ipc messaging.
}

type inventoryState struct {
	readyToReap    bool
	sendErrorCount uint32
}

var (
	alog  = log.WithComponent("Agent")
	aclog = log.WithComponent("AgentContext")
)

// AgentContext defines the interfaces between plugins and the agent
type AgentContext interface {
	Context() context2.Context
	SendData(types.PluginOutput)
	SendEvent(event sample.Event, entityKey entity.Key)
	Unregister(ids.PluginID)
	// Reconnecting tells the agent that this plugin must be re-executed when the agent reconnects after long time
	// disconnected (> 24 hours).
	AddReconnecting(Plugin)
	// Reconnect invokes again all the plugins that have been registered with the AddReconnecting function
	Reconnect()
	Config() *config.Config
	// EntityKey stores agent entity key (name), value may change in runtime.
	EntityKey() string
	Version() string

	// Service -> PID cache. This is used so we can translate between PIDs and service names easily.
	// The cache is populated by all plugins which produce lists of services, and used by metrics
	// which get processes and want to determine which service each process is for.
	CacheServicePids(source string, pidMap map[int]string)
	GetServiceForPid(pid int) (service string, ok bool)
	ActiveEntitiesChannel() chan string
	// HostnameResolver returns the host name resolver associated to the agent context
	HostnameResolver() hostname.Resolver
	IDLookup() host.IDLookup

	// Identity returns the entity ID of the infra agent
	Identity() entity.Identity
}

// context defines a bunch of agent data structures we make
// available to the various plugins and satisfies the
// AgentContext interface
type context struct {
	Ctx          context2.Context
	CancelFn     context2.CancelFunc
	cfg          *config.Config
	id           *id.Context
	agentKey     atomic.Value
	reconnecting *sync.Map               // Plugins that must be re-executed after a long disconnection
	ch           chan types.PluginOutput // Channel of inbound plugin data payloads

	updateIDLookupTableFn func(hostAliases types.PluginInventoryDataset) (err error)
	pluginOutputHandleFn  func(types.PluginOutput) // Function to handle the PluginOutput (Inventory Data). When this is provided the ch would not be used (In future would be deprecared)
	activeEntities        chan string              // Channel will be reported about the local/remote entities that are active
	version               string
	eventSender           eventSender

	servicePidLock     *sync.RWMutex
	servicePids        map[string]map[int]string // Map of plugin -> (map of pid -> service)
	resolver           hostname.ResolverChangeNotifier
	EntityMap          entity.KnownIDs
	idLookup           host.IDLookup
	shouldIncludeEvent sampler.IncludeSampleMatchFn
	shouldExcludeEvent sampler.ExcludeSampleMatchFn
}

func (c *context) Context() context2.Context {
	return c.Ctx
}

// AgentID provides agent ID, blocking until it's available
func (c *context) AgentID() entity.ID {
	return c.id.AgentID()
}

// AgentIdentity provides agent ID & GUID, blocking until it's available
func (c *context) Identity() entity.Identity {
	return c.id.AgentIdentity()
}

func (c *context) AgentIDUpdateNotifier() id.UpdateNotifyFn {
	return c.id.Notify
}

// AgentIDOrEmpty provides agent ID when available, empty otherwise
func (c *context) AgentIdnOrEmpty() entity.Identity {
	return c.id.AgentIdnOrEmpty()
}

func (c *context) IdContext() *id.Context {
	return c.id
}

// SetAgentID sets agent id
func (c *context) SetAgentIdentity(id entity.Identity) {
	c.id.SetAgentIdentity(id)
}

// IDLookup returns the IDLookup map.
func (c *context) IDLookup() host.IDLookup {
	return c.idLookup
}

// NewContext creates a new context.
func NewContext(
	cfg *config.Config,
	buildVersion string,
	resolver hostname.ResolverChangeNotifier,
	lookup host.IDLookup,
	sampleMatchFn sampler.IncludeSampleMatchFn,
	sampleExcludeFn sampler.ExcludeSampleMatchFn,
) *context {
	ctx, cancel := context2.WithCancel(context2.Background())

	var agentKey atomic.Value
	agentKey.Store("")
	return &context{
		cfg:                cfg,
		Ctx:                ctx,
		CancelFn:           cancel,
		id:                 id.NewContext(ctx),
		reconnecting:       new(sync.Map),
		version:            buildVersion,
		servicePidLock:     &sync.RWMutex{},
		servicePids:        make(map[string]map[int]string),
		resolver:           resolver,
		idLookup:           lookup,
		shouldIncludeEvent: sampleMatchFn,
		shouldExcludeEvent: sampleExcludeFn,
		agentKey:           agentKey,
	}
}

func checkCollectorConnectivity(ctx context2.Context, cfg *config.Config, retrier *backoff.RetryManager, userAgent string, agentKey string, transport http.RoundTripper) (err error) {
	if cfg.CollectorURL == "" {
		return
	}

	// if StartupConnectionRetries is negative, then we will keep checking the connection until it succeeds.
	tries := cfg.StartupConnectionRetries
	timeout, err := time.ParseDuration(cfg.StartupConnectionTimeout)
	if err != nil {
		// this should never happen, as the correct format is checked during NormalizeConfig
		return
	}
	var timedout bool

	for {
		timedout, err = backendhttp.CheckEndpointReachability(ctx, alog, cfg.CollectorURL, cfg.License, userAgent, agentKey, timeout, transport)
		if timedout {
			if tries >= 0 {
				tries -= 1
				if tries <= 0 {
					break
				}
			}
			alog.WithError(err).WithFields(logrus.Fields{
				"userAgent": userAgent,
				"timeout":   timeout,
				"url":       cfg.CollectorURL,
			}).Warn("no connectivity with collector url, retrying")
			retrier.SetNextRetryWithBackoff()
			time.Sleep(retrier.RetryAfter())
		} else {
			// otherwise we got a response, so break out
			break
		}
	}
	return
}

// NewAgent returns a new instance of an agent built from the config.
func NewAgent(
	cfg *config.Config,
	buildVersion string,
	userAgent string,
	ffRetriever feature_flags.Retriever,
) (a *Agent, err error) {
	hostnameResolver := hostname.CreateResolver(
		cfg.OverrideHostname, cfg.OverrideHostnameShort, cfg.DnsHostnameResolution)

	// Initialize the cloudDetector.
	cloudHarvester := cloud.NewDetector(cfg.DisableCloudMetadata, cfg.CloudMaxRetryCount, cfg.CloudRetryBackOffSec, cfg.CloudMetadataExpiryInSec, cfg.CloudMetadataDisableKeepAlive)
	cloudHarvester.Initialize(cloud.WithProvider(cloud.Type(cfg.CloudProvider)))

	idLookupTable := NewIdLookup(hostnameResolver, cloudHarvester, cfg.DisplayName)
	sampleMatchFn := sampler.NewSampleMatchFn(cfg.EnableProcessMetrics, cfg.IncludeMetricsMatchers, ffRetriever)
	sampleExcludeFn := sampler.NewSampleMatchFn(cfg.EnableProcessMetrics, cfg.ExcludeMetricsMatchers, ffRetriever)
	ctx := NewContext(cfg, buildVersion, hostnameResolver, idLookupTable, sampleMatchFn, sampleExcludeFn)

	agentKey, err := idLookupTable.AgentKey()
	if err != nil {
		return
	}
	ctx.setAgentKey(agentKey)

	var dataDir string
	if cfg.AppDataDir != "" {
		dataDir = filepath.Join(cfg.AppDataDir, "data")
	} else {
		dataDir = filepath.Join(cfg.AgentDir, "data")
	}

	maxInventorySize := cfg.MaxInventorySize
	if cfg.DisableInventorySplit {
		maxInventorySize = delta.DisableInventorySplit
	}

	s := delta.NewStore(dataDir, ctx.EntityKey(), maxInventorySize, cfg.InventoryArchiveEnabled)

	transport := backendhttp.BuildTransport(cfg, backendhttp.ClientTimeout)
	transport = backendhttp.NewRequestDecoratorTransport(cfg, transport)

	httpClient := backendhttp.GetHttpClient(backendhttp.ClientTimeout, transport)

	identityURL := fmt.Sprintf("%s/%s", cfg.IdentityURL, strings.TrimPrefix(cfg.IdentityIngestEndpoint, "/"))
	if os.Getenv("DEV_IDENTITY_INGEST_URL") != "" {
		identityURL = os.Getenv("DEV_IDENTITY_INGEST_URL")
	}
	identityURL = strings.TrimSuffix(identityURL, "/")
	connectClient, err := identityapi.NewIdentityConnectClient(
		identityURL,
		cfg.License,
		userAgent,
		cfg.PayloadCompressionLevel,
		cfg.IsContainerized,
		httpClient.Do,
	)
	if err != nil {
		return nil, err
	}

	registerClient, err := identityapi.NewRegisterClient(
		identityURL,
		cfg.License,
		userAgent,
		cfg.PayloadCompressionLevel,
		httpClient,
	)
	if err != nil {
		return nil, err
	}

	provideIDs := NewProvideIDs(registerClient, state.NewRegisterSM())
	fpHarvester, err := fingerprint.NewHarvestor(cfg, hostnameResolver, cloudHarvester)
	if err != nil {
		return nil, err
	}

	connectMetadataHarvester := identityapi.NewMetadataHarvesterDefault(hostid.NewProviderEnv())

	connectSrv := NewIdentityConnectService(connectClient, fpHarvester, connectMetadataHarvester)

	// notificationHandler will map ipc messages to functions
	notificationHandler := ctl.NewNotificationHandlerWithCancellation(ctx.Ctx)

	return New(
		cfg,
		ctx,
		userAgent,
		idLookupTable,
		s,
		connectSrv,
		provideIDs,
		httpClient.Do,
		transport,
		cloudHarvester,
		fpHarvester,
		notificationHandler,
	)
}

// New creates a new agent using given context and services.
func New(
	cfg *config.Config,
	ctx *context,
	userAgent string,
	idLookupTable host.IDLookup,
	s *delta.Store,
	connectSrv *identityConnectService,
	provideIDs ProvideIDs,
	dataClient backendhttp.Client,
	transport http.RoundTripper,
	cloudHarvester cloud.Harvester,
	fpHarvester fingerprint.Harvester,
	notificationHandler *ctl.NotificationHandlerWithCancellation,
) (*Agent, error) {
	a := &Agent{
		Context:             ctx,
		debugProvide:        debug.ProvideFn,
		userAgent:           userAgent,
		store:               s,
		httpClient:          dataClient,
		fpHarvester:         fpHarvester,
		cloudHarvester:      cloudHarvester,
		connectSrv:          connectSrv,
		provideIDs:          provideIDs,
		notificationHandler: notificationHandler,
	}

	a.plugins = make([]Plugin, 0)
	a.oldPlugins = make([]ids.PluginID, 0)

	a.Context.cfg = cfg
	a.agentDir = cfg.AgentDir
	if cfg.AppDataDir != "" {
		a.extDir = filepath.Join(cfg.AppDataDir, "user_data")
	} else {
		a.extDir = filepath.Join(a.agentDir, "user_data")
	}

	// register handlers for ipc messaging
	// for linux only "verbose logging" is handled
	notificationHandler.RegisterHandler(ipc.EnableVerboseLogging, a.enableVerboseLogging)
	notificationHandler.RegisterHandler(ipc.Stop, a.gracefulStop)
	notificationHandler.RegisterHandler(ipc.Shutdown, a.gracefulShutdown)

	// Instantiate reaper and sender
	a.inventories = map[string]*inventoryEntity{}

	// Make sure the network is working before continuing with identity
	if err := checkCollectorConnectivity(ctx.Ctx, cfg, backoff.NewRetrier(), a.userAgent, a.Context.getAgentKey(), transport); err != nil {
		alog.WithError(err).Error("network is not available")
		return nil, err
	}

	if err := a.setAgentKey(idLookupTable); err != nil {
		return nil, fmt.Errorf("could not determine any suitable identification for this agent. Attempts to gather the hostname, cloud ID, or configured alias all failed")
	}
	llog := alog.WithField("id", a.Context.EntityKey())
	llog.Debug("Bootstrap Entity Key.")

	// Create the external directory for user-generated json
	if err := disk.MkdirAll(a.extDir, 0o755); err != nil {
		llog.WithField("path", a.extDir).WithError(err).Error("External json directory could not be initialized")
		return nil, err
	}

	// Create input channel for plugins to feed data back to the agent
	llog.WithField(config.TracesFieldName, config.FeatureTrace).Tracef("inventory parallelize queue: %v", a.Context.cfg.InventoryQueueLen)
	a.Context.ch = make(chan types.PluginOutput, a.Context.cfg.InventoryQueueLen)
	a.Context.activeEntities = make(chan string, activeEntitiesBufferLength)

	if cfg.RegisterEnabled {
		localEntityMap := entity.NewKnownIDs()
		a.entityMap = localEntityMap
		a.Context.eventSender = newVortexEventSender(a.Context, cfg.License, a.userAgent, a.httpClient, a.provideIDs, localEntityMap)
	} else {
		a.Context.eventSender = newMetricsIngestSender(a.Context, cfg.License, a.userAgent, a.httpClient, cfg.ConnectEnabled)
	}

	return a, nil
}

// NewIdLookup creates a new agent ID lookup table.
func NewIdLookup(resolver hostname.Resolver, cloudHarvester cloud.Harvester, displayName string) host.IDLookup {
	idLookupTable := make(host.IDLookup)
	// Attempt to get the hostname
	host, short, err := resolver.Query()
	llog := alog.WithField("displayName", displayName)
	if err == nil {
		idLookupTable[sysinfo.HOST_SOURCE_HOSTNAME] = host
		idLookupTable[sysinfo.HOST_SOURCE_HOSTNAME_SHORT] = short
	} else {
		llog.WithError(err).Warn("could not determine hostname")
	}
	if host == "localhost" {
		llog.Warn("Localhost is not a good identifier")
	}
	// See if we have a configured alias which is not equal to the hostname, if so, use
	// it as a unique identifier and ignore the hostname
	if displayName != "" {
		idLookupTable[sysinfo.HOST_SOURCE_DISPLAY_NAME] = displayName
	}
	cloudInstanceID, err := cloudHarvester.GetInstanceID()
	if err != nil {
		llog.WithField("idLookupTable", idLookupTable).WithError(err).Debug("Unable to get instance id.")
	} else {
		idLookupTable[sysinfo.HOST_SOURCE_INSTANCE_ID] = cloudInstanceID
	}

	return idLookupTable
}

// Instantiates delta.Store as well as associated reapers and senders
func (a *Agent) registerEntityInventory(entity entity.Entity) error {
	entityKey := entity.Key.String()

	alog.WithField("entityKey", entityKey).
		WithField("entityID", entity.ID).Debug("Registering inventory for entity.")

	var patchSender inventory.PatchSender
	var err error
	if a.Context.cfg.RegisterEnabled {
		patchSender, err = newPatchSenderVortex(entityKey, a.Context.getAgentKey(), a.Context, a.store, a.userAgent, a.Context.Identity, a.provideIDs, a.entityMap, a.httpClient)
	} else {
		patchSender, err = a.newPatchSender(entity)
	}
	if err != nil {
		return err
	}

	reaper := newPatchReaper(entityKey, a.store)
	a.inventories[entityKey] = &inventoryEntity{
		sender: patchSender,
		reaper: reaper,
	}

	return nil
}

func (a *Agent) newPatchSender(entity entity.Entity) (inventory.PatchSender, error) {
	fileName := a.store.EntityFolder(entity.Key.String())
	lastSubmission := delta.NewLastSubmissionStore(a.store.DataDir, fileName)
	lastEntityID := delta.NewEntityIDFilePersist(a.store.DataDir, fileName)

	return newPatchSender(entity, a.Context, a.store, lastSubmission, lastEntityID, a.userAgent, a.Context.Identity, a.httpClient)
}

// removes the inventory object references to free the memory, and the respective directories
func (a *Agent) unregisterEntityInventory(entityKey string) error {
	alog.WithField("entityKey", entityKey).Debug("Unregistering inventory for entity.")

	_, ok := a.inventories[entityKey]
	if ok {
		delete(a.inventories, entityKey)
	}

	return a.store.RemoveEntity(entityKey)
}

func (a *Agent) Plugins() []Plugin {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.plugins
}

// Terminate takes all the reactive actions required before a termination (e.g. killing all the running processes)
func (a *Agent) Terminate() {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	alog.Debug("Terminating running plugins.")
	for _, agentPlugin := range a.plugins {
		switch p := agentPlugin.(type) {
		case Killable:
			p.Kill()
		}
	}
}

func (a *Agent) RegisterMetricsSender(s registerableSender) {
	a.metricsSender = s
}

// RegisterPlugin takes a Plugin instance and registers it in the
// agent's plugin map
func (a *Agent) RegisterPlugin(p Plugin) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.plugins = append(a.plugins, p)
}

// ExternalPluginsHealthCheck schedules the plugins health checks.
func (a *Agent) ExternalPluginsHealthCheck() {
	for _, p := range a.plugins {
		p.ScheduleHealthCheck()
	}
}

func (a *Agent) GetContext() AgentContext {
	return a.Context
}

// GetCloudHarvester will return the CloudHarvester service.
func (a *Agent) GetCloudHarvester() cloud.Harvester {
	return a.cloudHarvester
}

// DeprecatePlugin builds the list of deprecated plugins
func (a *Agent) DeprecatePlugin(plugin ids.PluginID) {
	a.oldPlugins = append(a.oldPlugins, plugin)
}

// storePluginOutput will take a PluginOutput and persist it in the store
func (a *Agent) storePluginOutput(pluginOutput types.PluginOutput) error {
	if pluginOutput.Data == nil {
		pluginOutput.Data = make(types.PluginInventoryDataset, 0)
	}

	sort.Sort(pluginOutput.Data)

	// Filter out ignored inventory data before writing the file out
	var sortKey string
	ignore := a.Context.Config().IgnoredInventoryPathsMap
	simplifiedPluginData := make(map[string]interface{})
DataLoop:
	for _, data := range pluginOutput.Data {
		if data == nil {
			continue
		}
		sortKey = data.SortKey()
		pluginSource := fmt.Sprintf("%s/%s", pluginOutput.Id, sortKey)
		if _, ok := ignore[strings.ToLower(pluginSource)]; ok {
			continue DataLoop
		}
		simplifiedPluginData[sortKey] = data
	}

	return a.store.SavePluginSource(
		pluginOutput.Entity.Key.String(),
		pluginOutput.Id.Category,
		pluginOutput.Id.Term,
		simplifiedPluginData,
	)
}

// startPlugins takes all the registered plugins and starts them up serially
// we don't return until all plugins have been started
func (a *Agent) startPlugins() {
	// iterate over and start each plugin
	for _, agentPlugin := range a.plugins {
		agentPlugin.LogInfo()
		go func(p Plugin) {
			_, trx := instrumentation.SelfInstrumentation.StartTransaction(context2.Background(), fmt.Sprintf("plugin. %s ", p.Id().String()))
			defer trx.End()
			p.Run()
		}(agentPlugin)
	}
}

// LogExternalPluginsInfo iterates over the list of plugins and logs
// the information of the external plugins only.
func (a *Agent) LogExternalPluginsInfo() {
	for _, plugin := range a.plugins {
		if plugin.IsExternal() {
			plugin.LogInfo()
		}
	}
}

var hostAliasesPluginID = ids.PluginID{Category: "metadata", Term: "host_aliases"}

func (a *Agent) updateIDLookupTable(hostAliases types.PluginInventoryDataset) (err error) {
	newIDLookupTable := make(map[string]string)
	for _, hAliases := range hostAliases {
		if alias, ok := hAliases.(sysinfo.HostAliases); ok {
			newIDLookupTable[alias.Source] = alias.Alias
		}
	}
	_ = a.setAgentKey(newIDLookupTable)
	return
}

// Given a map of ID types to ID values, this will go through our list of preferred
// identifiers and choose the first one we can find.
// If one is found, it will be set on the context object as well as returned.
// Otherwise, the context is not modified.
func (a *Agent) setAgentKey(idLookupTable host.IDLookup) error {
	key, err := idLookupTable.AgentKey()
	if err != nil {
		return err
	}

	alog.WithField("old", a.Context.getAgentKey()).WithField("new", key).Debug("Updating identity.")

	a.Context.setAgentKey(key)

	if a.store != nil {
		a.store.ChangeDefaultEntity(key)
	}

	return nil
}

func (a *Agent) Init() {
	cfg := a.Context.cfg

	// Configure AsyncInventoryHandler if FF is enabled.
	if cfg.AsyncInventoryHandlerEnabled {
		alog.Debug("Initialise async inventory handler")

		removeEntitiesPeriod, _ := time.ParseDuration(a.Context.Config().RemoveEntitiesPeriod)

		patcherConfig := inventory.PatcherConfig{
			IgnoredPaths:         cfg.IgnoredInventoryPathsMap,
			AgentEntity:          entity.NewFromNameWithoutID(a.Context.EntityKey()),
			RemoveEntitiesPeriod: removeEntitiesPeriod,
		}
		patcher := inventory.NewEntityPatcher(patcherConfig, a.store, a.newPatchSender)

		if cfg.InventoryQueueLen == 0 {
			cfg.InventoryQueueLen = defaultBulkInventoryQueueLength
		}

		inventoryHandlerCfg := inventory.HandlerConfig{
			SendInterval:      cfg.SendInterval,
			FirstReapInterval: cfg.FirstReapInterval,
			ReapInterval:      cfg.ReapInterval,
			InventoryQueueLen: cfg.InventoryQueueLen,
		}
		a.inventoryHandler = inventory.NewInventoryHandler(a.Context.Ctx, inventoryHandlerCfg, patcher)
		a.Context.pluginOutputHandleFn = a.inventoryHandler.Handle
		a.Context.updateIDLookupTableFn = a.updateIDLookupTable

		// When AsyncInventoryHandlerEnabled is set disable inventory archiving.
		a.store.SetArchiveEnabled(false)
	}
}

// Run is the main event loop for the agent it starts up the plugins
// kicks off a filesystem seed and watcher and listens for data from
// the plugins
func (a *Agent) Run() (err error) {
	alog.Info("Starting up agent...")
	// start listening for ipc messages
	_ = a.notificationHandler.Start()

	cfg := a.Context.cfg

	f := a.cpuProfileStart()
	if f != nil {
		defer a.cpuProfileStop(f)
	}

	go a.intervalMemoryProfile()

	if cloud.Type(cfg.CloudProvider).IsValidCloud() {
		err = a.checkInstanceIDRetry(cfg.CloudMaxRetryCount, cfg.CloudRetryBackOffSec)
		// If the cloud provider was specified but we cannot get the instance ID, agent fails
		if err != nil {
			alog.WithError(err).Error("Couldn't detect the instance ID for the specified cloud")

			return
		}
	}

	if cfg.ConnectEnabled {
		go a.connect()
	}

	alog.Debug("Starting Plugins.")
	a.startPlugins()

	if err != nil {
		alog.WithError(err).Error("failed to start troubleshooting handler")
	}

	// Start debugger routine.
	go func() {
		if a.Context.Config().DebugLogSec <= 0 {
			return
		}

		debugTimer := time.NewTicker(time.Duration(a.Context.Config().DebugLogSec) * time.Second)

		for {
			select {
			case <-debugTimer.C:
				{
					debugInfo, err := a.debugProvide()
					if err != nil {
						alog.WithError(err).Debug("failed to get debug stats")
					} else if debugInfo != "" {
						alog.Debug(debugInfo)
					}
				}
			case <-a.Context.Ctx.Done():
				debugTimer.Stop()

				return
			}
		}
	}()

	if a.Context.eventSender != nil {
		if err := a.Context.eventSender.Start(); err != nil {
			alog.WithError(err).Error("failed to start event sender")
		}
	}

	if a.metricsSender != nil {
		if err := a.metricsSender.Start(); err != nil {
			alog.WithError(err).Error("failed to start metrics subsystem")
		}
	}

	exit := make(chan struct{})

	go func() {
		<-a.Context.Ctx.Done()

		a.exitGracefully()

		close(exit)
	}()

	if a.inventoryHandler != nil {
		if a.shouldSendInventory() {
			a.inventoryHandler.Start()
		}
		<-exit

		return nil
	}

	a.handleInventory(exit)

	return nil
}

func (a *Agent) handleInventory(exit chan struct{}) {
	cfg := a.Context.cfg

	// Timers
	reapInventoryTimer := time.NewTicker(cfg.FirstReapInterval)
	sendInventoryTimer := time.NewTimer(cfg.SendInterval) // Send any deltas every X seconds

	// Remove send timer
	if !a.shouldSendInventory() {
		// If Stop returns false means that the timer has been already triggered
		if !sendInventoryTimer.Stop() {
			<-sendInventoryTimer.C
		}
		reapInventoryTimer.Stop()
		alog.Info("inventory submission disabled")
	}

	// Timer to engage the process of deleting entities that haven't been reported information during this time
	removeEntitiesPeriod, err := time.ParseDuration(a.Context.Config().RemoveEntitiesPeriod)
	if removeEntitiesPeriod <= 0 || err != nil {
		removeEntitiesPeriod = defaultRemoveEntitiesPeriod
		err = nil
	}

	removeEntitiesTicker := time.NewTicker(removeEntitiesPeriod)
	reportedEntities := map[string]bool{}

	// Wait no more than this long for initial inventory reap even if some plugins haven't reported data
	initialReapTimeout := time.NewTimer(config.INITIAL_REAP_MAX_WAIT_SECONDS * time.Second)

	// keep track of which plugins have phone home
	idsReporting := make(map[ids.PluginID]bool)
	distinctPlugins := make(map[ids.PluginID]Plugin)
	for _, p := range a.plugins {
		distinctPlugins[p.Id()] = p
	}

	// Register local entity inventory
	// This will make the agent submitting unsent deltas from a previous execution (e.g. if an inventory was reaped
	// but the agent was restarted before sending it)
	if _, ok := a.inventories[a.Context.EntityKey()]; !ok {
		_ = a.registerEntityInventory(entity.NewFromNameWithoutID(a.Context.EntityKey()))
	}

	// three states
	//  -- reading data to write to json
	//  -- reaping
	//  -- sending
	// ready to consume events
	for {
		select {
		case <-exit:
			if sendInventoryTimer != nil {
				sendInventoryTimer.Stop()
			}
			if reapInventoryTimer != nil {
				reapInventoryTimer.Stop()
			}
			if removeEntitiesTicker != nil {
				removeEntitiesTicker.Stop()
			}
			return
			// agent gets notified about active entities
		case ent := <-a.Context.activeEntities:
			reportedEntities[ent] = true
			// read data from plugin and write json
		case data := <-a.Context.ch:
			{
				idsReporting[data.Id] = true

				if data.Id == hostAliasesPluginID {
					_ = a.updateIDLookupTable(data.Data)
				}

				if !data.NotApplicable {
					entityKey := data.Entity.Key.String()
					if _, ok := a.inventories[entityKey]; !ok {
						_ = a.registerEntityInventory(data.Entity)
					}

					if err := a.storePluginOutput(data); err != nil {
						alog.WithError(err).Error("problem storing plugin output")
					}
					a.inventories[entityKey].needsReaping = true
				}
			}
		case <-reapInventoryTimer.C:
			{
				for _, inventory := range a.inventories {
					if !a.inv.readyToReap {
						if len(distinctPlugins) <= len(idsReporting) {
							alog.Debug("Signalling initial reap.")
							a.inv.readyToReap = true
							inventory.needsCleanup = true
						} else {
							pluginIds := make([]ids.PluginID, 0)
							for plgId := range distinctPlugins {
								if !idsReporting[plgId] {
									pluginIds = append(pluginIds, plgId)
								}
							}
							alog.WithField("pluginIds", pluginIds).Debug("Still waiting on plugins.")
						}
					}
					if a.inv.readyToReap && inventory.needsReaping {
						reapInventoryTimer.Stop()
						reapInventoryTimer = time.NewTicker(cfg.ReapInterval)
						inventory.reaper.Reap()
						if inventory.needsCleanup {
							inventory.reaper.CleanupOldPlugins(a.oldPlugins)
							inventory.needsCleanup = false
						}
						inventory.needsReaping = false
					}
				}
			}
		case <-initialReapTimeout.C:
			// If we've waited too long and still not received data from all plugins, we can just send what we have.
			if !a.inv.readyToReap {
				alog.Debug("Maximum initial reap delay exceeded - marking inventory as ready to report.")
				a.inv.readyToReap = true
				for _, inventory := range a.inventories {
					inventory.needsCleanup = true
				}
			}
		case <-sendInventoryTimer.C:
			a.sendInventory(sendInventoryTimer)
		case <-removeEntitiesTicker.C:
			pastPeriodReportedEntities := reportedEntities
			reportedEntities = map[string]bool{} // reset the set of reporting entities the next period
			alog.Debug("Triggered periodic removal of outdated entities.")
			a.removeOutdatedEntities(pastPeriodReportedEntities)
		}
	}
}

// checkInstanceIDRetry will try to read the cloud instance ID until maxRetries is reached.
func (a *Agent) checkInstanceIDRetry(maxRetries, backoffTime int) error {
	var err error
	for i := 0; i <= maxRetries; i++ {
		if _, err = a.cloudHarvester.GetInstanceID(); err == nil {
			return nil
		}

		if i >= maxRetries-1 {
			break
		}

		alog.WithError(err).Debugf("Failed to get the instance ID, retrying in %d s.", backoffTime)
		time.Sleep(time.Duration(backoffTime) * time.Second)
	}

	return fmt.Errorf("failed to get an instance ID after %d attempt(s): %w", maxRetries+1, err)
}

func (a *Agent) cpuProfileStart() *os.File {
	// Start CPU profiling
	if a.Context.cfg.CPUProfile == "" {
		return nil
	}

	clog.Debug("Starting CPU profiling.")
	f, err := os.Create(a.Context.cfg.CPUProfile)
	if err != nil {
		clog.WithError(err).Error("could not create CPU profile file")
		return nil
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		clog.WithError(err).Error("could not start CPU profile")
		helpers.CloseQuietly(f)
		return nil
	}
	return f
}

func (a *Agent) cpuProfileStop(f *os.File) {
	clog := alog.WithField("cpuProfile", a.Context.cfg.CPUProfile)
	clog.Debug("Stopping CPU profiling.")
	pprof.StopCPUProfile()
	helpers.CloseQuietly(f)
}

func (a *Agent) intervalMemoryProfile() {
	cfg := a.Context.cfg

	if cfg.MemProfileInterval <= 0 {
		return
	}

	ticker := time.NewTicker(time.Second * time.Duration(cfg.MemProfileInterval))

	counter := 1

	for {
		select {
		case <-ticker.C:
			a.dumpMemoryProfile(counter * cfg.MemProfileInterval)
			counter++
		}
	}
}

func (a *Agent) dumpMemoryProfile(agentRuntimeMark int) {
	if a.Context.cfg.MemProfile == "" {
		return
	}
	memProfileFilename := fmt.Sprintf("%s_%09ds", a.Context.cfg.MemProfile, agentRuntimeMark)

	mlog := alog.WithField("memProfile", memProfileFilename)
	mlog.Debug("Starting memory profiling.")
	f, err := os.Create(memProfileFilename)
	if err != nil {
		mlog.WithError(err).Error("could not create memory profile file")
	} else {
		defer helpers.CloseQuietly(f)
		runtime.GC()                  // get up-to-date statistics
		runtime.MemProfileRate = 1024 // trigger alloc profile in a per MB basis
		if err := pprof.WriteHeapProfile(f); err != nil {
			mlog.WithError(err).Error("could not start memory profile")
		}
	}
}

func (a *Agent) exitGracefully() {
	log.Info("Gracefully Exiting")

	if a.Context.eventSender != nil {
		if err := a.Context.eventSender.Stop(); err != nil {
			log.WithError(err).Error("failed to stop event sender")
		}
	}
	if a.metricsSender != nil {
		if err := a.metricsSender.Stop(); err != nil {
			log.WithError(err).Error("failed to stop metrics subsystem")
		}
	}

	if a.inventoryHandler != nil {
		a.inventoryHandler.Stop()
	}

	if a.notificationHandler != nil {
		a.notificationHandler.Stop()
	}
}

func (a *Agent) sendInventory(sendTimer *time.Timer) {
	backoffMax := config.MAX_BACKOFF
	for _, i := range a.inventories {
		err := i.sender.Process()
		if err != nil {
			if ingestError, ok := err.(*inventoryapi.IngestError); ok &&
				ingestError.StatusCode == http.StatusTooManyRequests {
				alog.Warn("server is rate limiting inventory submission")
				backoffMax = config.RATE_LIMITED_BACKOFF
				a.inv.sendErrorCount = helpers.MaxBackoffErrorCount
			} else {
				a.inv.sendErrorCount++
			}
			alog.WithError(err).WithField("errorCount", a.inv.sendErrorCount).
				Debug("Inventory sender can't process after retrying.")
			// Assuming break will try to send later the data from the missing inventory senders
			break
		} else {
			a.inv.sendErrorCount = 0
		}
	}
	sendTimerVal := helpers.ExpBackoff(a.Context.cfg.SendInterval,
		time.Duration(backoffMax)*time.Second,
		a.inv.sendErrorCount)
	sendTimer.Reset(sendTimerVal)
}

func (a *Agent) removeOutdatedEntities(reportedEntities map[string]bool) {
	alog.Debug("Triggered periodic removal of outdated entities.")
	// The entities to remove are those entities that haven't reported activity in the last period and
	// are registered in the system
	entitiesToRemove := map[string]bool{}
	for entityKey := range a.inventories {
		entitiesToRemove[entityKey] = true
	}
	delete(entitiesToRemove, a.Context.EntityKey()) // never delete local entity
	for entityKey := range reportedEntities {
		delete(entitiesToRemove, entityKey)
	}
	for entityKey := range entitiesToRemove {
		elog := alog.WithField("entityKey", entityKey)
		elog.Debug("Removing inventory for entity.")
		if err := a.unregisterEntityInventory(entityKey); err != nil {
			elog.WithError(err).Warn("unregistering inventory for entity")
		}
	}
	// Remove folders from unregistered entities that still have folders in the data directory (e.g. from
	// previous agent executions)
	foldersToRemove, err := a.store.ScanEntityFolders()
	if err != nil {
		alog.WithError(err).Warn("error scanning outdated entity folders")
		// Continuing, because some entities may have been fetched despite the error
	}
	if foldersToRemove != nil {
		// We don't remove those entities that are registered
		for entityKey := range a.inventories {
			delete(foldersToRemove, helpers.SanitizeFileName(entityKey))
		}
		for folder := range foldersToRemove {
			if err := a.store.RemoveEntityFolders(folder); err != nil {
				alog.WithField("folder", folder).WithError(err).Warn("error removing entity folder")
			}
		}
	}

	alog.WithField("remaining", len(a.inventories)).Debug("Some entities may remain registered.")
}

func (c *context) SendData(data types.PluginOutput) {
	if c.pluginOutputHandleFn != nil {
		if data.Id == hostAliasesPluginID && c.updateIDLookupTableFn != nil {
			c.updateIDLookupTableFn(data.Data)
		}
		c.pluginOutputHandleFn(data)
		return
	}
	c.ch <- data
}

func (c *context) ActiveEntitiesChannel() chan string {
	return c.activeEntities
}

func (c *context) SendEvent(event sample.Event, entityKey entity.Key) {
	_, txn := instrumentation.SelfInstrumentation.StartTransaction(context2.Background(), "agent.queue_event")
	defer txn.End()

	if c.eventSender == nil {
		aclog.
			WithField("entity_key", entityKey.String()).
			Warn("cannot send, event sender not set")
		return
	}

	// truncates string fields larger than 4095 chars
	if c.cfg.TruncTextValues {
		var truncated bool
		origValue := fmt.Sprintf("+%v", event)
		event, truncated = metric.TruncateLength(event, metric.NRDBLimit)
		if truncated {
			aclog.
				WithField("entity_key", entityKey.String()).
				WithField("length", len(origValue)).
				WithField("original", origValue).
				WithField("truncated", fmt.Sprintf("+%v", event)).
				Warn("event truncated to NRDB limit")
		}
	}

	// check if event should be included
	// include takes precedence, so the event will be included if
	// it IS NOT EXCLUDED or if it IS INCLUDED
	includeSample := !c.shouldExcludeEvent(event) || c.shouldIncludeEvent(event)
	if !includeSample {
		aclog.
			WithField("entity_key", entityKey.String()).
			WithField("event", fmt.Sprintf("+%v", event)).
			Debug("event excluded by metric matcher")
		return
	}

	if err := c.eventSender.QueueEvent(event, entityKey); err != nil {
		txn.NoticeError(err)
		alog.WithField(
			"entityKey", entityKey,
		).WithError(err).Error("could not queue event")
	}
}

func (c *context) Unregister(id ids.PluginID) {
	c.ch <- types.NewNotApplicableOutput(id)
}

func (c *context) Config() *config.Config {
	return c.cfg
}

func (c *context) EntityKey() string {
	return c.getAgentKey()
}

func (c *context) Version() string {
	return c.version
}

func (c *context) CacheServicePids(source string, pidMap map[int]string) {
	newPidMap := make(map[int]string)
	for pid, service := range pidMap {
		newPidMap[pid] = service
	}

	c.servicePidLock.Lock()
	defer c.servicePidLock.Unlock()
	c.servicePids[source] = newPidMap
}

func (c *context) GetServiceForPid(pid int) (service string, ok bool) {
	c.servicePidLock.RLock()
	defer c.servicePidLock.RUnlock()

	for _, pidSourceName := range sysinfo.PROCESS_NAME_SOURCES {
		if sourcePidMap, ok := c.servicePids[pidSourceName]; ok {
			if service, ok := sourcePidMap[pid]; ok {
				return service, true
			}
		}
	}

	return
}

// Reconnecting tells the agent that this plugin must be re-executed when the agent reconnects after long time
// disconnected (> 24 hours).
func (c *context) AddReconnecting(p Plugin) {
	c.reconnecting.Store(p.Id(), p)
}

// Reconnect invokes again all the plugins that have been registered with the AddReconnecting function
func (c *context) Reconnect() {
	aclog.Debug("Invoking Run() on all the plugins registered for reconnection.")
	c.reconnecting.Range(triggerAddReconnecting(aclog))
}

// triggerAddReconnecting is used with sync.Map.Range to iterate through all plugins and reconnect them
func triggerAddReconnecting(l log.Entry) func(pluginID interface{}, plugin interface{}) bool {
	return func(pluginID, agentPlugin interface{}) bool {
		l.WithField("plugin", pluginID).Debug("Reconnecting plugin.")
		func(p Plugin) {
			go p.Run()
		}(agentPlugin.(Plugin))
		return true
	}
}

// HostnameResolver returns the host name resolver associated to the agent context
func (c *context) HostnameResolver() hostname.Resolver {
	return c.resolver
}

// HostnameResolver returns the host name change notifier associated to the agent context
func (c *context) HostnameChangeNotifier() hostname.ChangeNotifier {
	return c.resolver
}

func (c *context) setAgentKey(agentKey string) {
	c.agentKey.Store(agentKey)
}

func (c *context) getAgentKey() (agentKey string) {
	loaded := c.agentKey.Load()
	if loaded == nil {
		return ""
	}
	return c.agentKey.Load().(string)
}

func (a *Agent) connect() {
	alog.Debug("Performing connect.")
	a.Context.SetAgentIdentity(a.connectSrv.Connect())

	updateFreq := time.Duration(a.Context.cfg.FingerprintUpdateFreqSec) * time.Second
	ticker := time.NewTicker(updateFreq)

	for range ticker.C {
		alog.Debug("Performing connect update.")
		identity, err := a.connectSrv.ConnectUpdate(a.Context.Identity())
		if err != nil {
			alog.WithError(err).Warn("error occurred while updating the system fingerprint")
			continue
		}
		a.Context.SetAgentIdentity(identity)
	}
}

func (a *Agent) enableVerboseLogging() error {
	alog.Debug("Enabling temporary verbose logging.")
	log.EnableTemporaryVerbose()

	a.LogExternalPluginsInfo()
	a.Context.cfg.LogInfo()
	a.ExternalPluginsHealthCheck()
	return nil
}

// this function is only called in the Windows implementation when the agent service is being
// stopped from Services management console
func (a *Agent) gracefulStop() error {
	alog.Info("stopping gracefully...")
	// Stop timers et al
	a.Context.CancelFn()
	return nil
}

// this function is only called inn the Windows implementation when the agent service is being
// stopped due to a OS shutdown/restart
func (a *Agent) gracefulShutdown() error {
	alog.Info("shutting down gracefully...")
	// Stop timers et al

	defer a.Context.CancelFn()

	return a.connectSrv.Disconnect(a.Context.AgentID(), identityapi.ReasonHostShutdown)
}

func (a *Agent) shouldSendInventory() bool {
	return !a.GetContext().Config().IsForwardOnly
}
