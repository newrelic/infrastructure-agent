// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	context2 "context"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/helpers/metric"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/ctl"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"

	"github.com/pkg/errors"

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
	defaultRemoveEntitiesPeriod = 48 * time.Hour
	activeEntitiesBufferLength  = 32
)

// ErrUndefinedLookupType error when an identifier type is not found
var ErrUndefinedLookupType = errors.New("no known identifier types found in ID lookup table")

type registerableSender interface {
	Start() error
	Stop() error
}

type Agent struct {
	plugins             []Plugin              // Slice of registered plugins
	oldPlugins          []ids.PluginID        // Deprecated plugins whose cached data must be removed, if existing
	agentDir            string                // Base data directory for the agent
	extDir              string                // Location of external data input
	userAgent           string                // User-Agent making requests to warlock
	inventories         map[string]*inventory // Inventory reaper and sender instances (key: entity ID)
	Context             *context              // Agent context data that is passed around the place
	metricsSender       registerableSender
	store               *delta.Store
	lastSubmissionStore delta.LastSubmissionStore
	debugProvide        debug.Provide
	httpClient          backendhttp.Client // http client for both data submission types: events and inventory

	connectSrv *identityConnectService

	provideIDs ProvideIDs
	entityMap  entity.KnownIDs

	fpHarvester         fingerprint.Harvester
	cloudHarvester      cloud.Harvester                          // If it's the case returns information about the cloud where instance is running.
	agentID             *entity.ID                               // pointer as it's referred from several points
	mtx                 sync.Mutex                               // Protect plugins
	notificationHandler *ctl.NotificationHandlerWithCancellation // Handle ipc messaging.
}

// inventory holds the reaper and sender for the inventories of a given entity (local or remote), as well as their status
type inventory struct {
	reaper       *patchReaper
	sender       patchSender
	needsReaping bool
	needsCleanup bool
}

var alog = log.WithComponent("Agent")

// AgentContext defines the interfaces between plugins and the agent
type AgentContext interface {
	SendData(PluginOutput)
	SendEvent(event sample.Event, entityKey entity.Key)
	Unregister(ids.PluginID)
	// Reconnecting tells the agent that this plugin must be re-executed when the agent reconnects after long time
	// disconnected (> 24 hours).
	AddReconnecting(Plugin)
	// Reconnect invokes again all the plugins that have been registered with the AddReconnecting function
	Reconnect()
	Config() *config.Config
	// AgentIdentifier value may change in runtime
	AgentIdentifier() string
	Version() string

	// Service -> PID cache. This is used so we can translate between PIDs and service names easily.
	// The cache is populated by all plugins which produce lists of services, and used by metrics
	// which get processes and want to determine which service each process is for.
	CacheServicePids(source string, pidMap map[int]string)
	GetServiceForPid(pid int) (service string, ok bool)
	ActiveEntitiesChannel() chan string
	// HostnameResolver returns the host name resolver associated to the agent context
	HostnameResolver() hostname.Resolver
	IDLookup() IDLookup
}

// context defines a bunch of agent data structures we make
// available to the various plugins and satisfies the
// AgentContext interface
type context struct {
	Ctx            context2.Context
	CancelFn       context2.CancelFunc
	cfg            *config.Config
	id             *id.Context
	agentKey       string
	reconnecting   *sync.Map         // Plugins that must be re-executed after a long disconnection
	ch             chan PluginOutput // Channel of inbound plugin data payloads
	activeEntities chan string       // Channel will be reported about the local/remote entities that are active
	version        string
	eventSender    eventSender

	servicePidLock     *sync.RWMutex
	servicePids        map[string]map[int]string // Map of plugin -> (map of pid -> service)
	resolver           hostname.ResolverChangeNotifier
	EntityMap          entity.KnownIDs
	idLookup           IDLookup
	shouldIncludeEvent includeSampleMatcher
}

// func that satisfies the metrics matcher (processor.MatcherChain) interface while avoiding the import
type includeSampleMatcher func(sample interface{}) bool

// AgentID provides agent ID, blocking until it's available
func (c *context) AgentID() entity.ID {
	return c.id.AgentID()
}

// AgentIdentity provides agent ID & GUID, blocking until it's available
func (c *context) AgentIdentity() entity.Identity {
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

//IDLookup returns the IDLookup map.
func (c *context) IDLookup() IDLookup {
	return c.idLookup
}

// IDLookup contains the identifiers used for resolving the agent entity name and agent key.
type IDLookup map[string]string

//NewContext creates a new context.
func NewContext(cfg *config.Config, buildVersion string, resolver hostname.ResolverChangeNotifier, lookup IDLookup,
	sampleMatcher includeSampleMatcher) *context {
	ctx, cancel := context2.WithCancel(context2.Background())

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
		shouldIncludeEvent: sampleMatcher,
	}
}

func checkEndpointAvailability(ctx context2.Context, cfg *config.Config, userAgent, agentKey string, timeout time.Duration, transport *http.Transport) (timedout bool, err error) {
	var request *http.Request
	if request, err = http.NewRequest("HEAD", cfg.CollectorURL, nil); err != nil {
		return false, fmt.Errorf("unable to prepare availability request: %v", request)
	}

	request = request.WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set(backendhttp.LicenseHeader, cfg.License)
	request.Header.Set(backendhttp.EntityKeyHeader, agentKey)

	client := backendhttp.GetHttpClient(timeout, transport)

	if _, err = client.Do(request); err != nil {
		if e2, ok := err.(net.Error); ok && (e2.Timeout() || e2.Temporary()) {
			timedout = true
		}
		if _, ok := err.(*url.Error); ok {
			alog.WithError(err).WithFields(logrus.Fields{
				"userAgent": userAgent,
				"timeout":   timeout,
				"url":       cfg.CollectorURL,
			}).Debug("URL Error detected, may be configuration problem or network connectivity issue.")
			timedout = true
		}
	}

	return
}

func checkCollectorConnectivity(ctx context2.Context, cfg *config.Config, retrier *backoff.RetryManager, userAgent string, agentKey string, transport *http.Transport) (err error) {
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
		timedout, err = checkEndpointAvailability(ctx, cfg, userAgent, agentKey, timeout, transport)
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

func getSampleMatcher(c *config.Config) func(interface{}) bool {
	ec := sampler.NewMatcherChain(c.IncludeMetricsMatchers)
	if ec.Enabled {
		return func(sample interface{}) bool {
			return ec.Evaluate(sample)
		}
	}

	alog.Debug("Evaluation chain is DISABLED, using default behaviour")

	// default matching function. All samples/event will be included
	return func(sample interface{}) bool {
		return true
	}
}

// NewAgent returns a new instance of an agent built from the config.
func NewAgent(cfg *config.Config, buildVersion string) (a *Agent, err error) {

	hostnameResolver := hostname.CreateResolver(
		cfg.OverrideHostname, cfg.OverrideHostnameShort, cfg.DnsHostnameResolution)

	// Initialize the cloudDetector.
	cloudHarvester := cloud.NewDetector(cfg.DisableCloudMetadata, cfg.CloudMaxRetryCount, cfg.CloudRetryBackOffSec, cfg.CloudMetadataExpiryInSec, cfg.CloudMetadataDisableKeepAlive)
	cloudHarvester.Initialize()

	idLookupTable := NewIdLookup(hostnameResolver, cloudHarvester, cfg.DisplayName)
	sampleMatcher := getSampleMatcher(cfg)
	ctx := NewContext(cfg, buildVersion, hostnameResolver, idLookupTable, sampleMatcher)

	agentKey, err := idLookupTable.getAgentKey()
	if err != nil {
		return
	}
	ctx.agentKey = agentKey

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

	s := delta.NewStore(dataDir, ctx.AgentIdentifier(), maxInventorySize)

	lastInventorySubmissionStore := delta.NewLastSubmissionStore(dataDir)

	userAgent := GenerateUserAgent("New Relic Infrastructure Agent", buildVersion)

	transport := backendhttp.BuildTransport(cfg, backendhttp.ClientTimeout)

	httpClient := backendhttp.GetHttpClient(backendhttp.ClientTimeout, transport).Do

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
		httpClient,
	)
	if err != nil {
		return nil, err
	}

	registerClient, err := identityapi.NewIdentityRegisterClient(
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

	connectSrv := NewIdentityConnectService(connectClient, fpHarvester)

	// notificationHandler will map ipc messages to functions
	notificationHandler := ctl.NewNotificationHandlerWithCancellation(ctx.Ctx)

	return New(
		cfg,
		ctx,
		userAgent,
		idLookupTable,
		s,
		lastInventorySubmissionStore,
		connectSrv,
		provideIDs,
		httpClient,
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
	idLookupTable IDLookup,
	s *delta.Store,
	lastInventoryStore delta.LastSubmissionStore,
	connectSrv *identityConnectService,
	provideIDs ProvideIDs,
	dataClient backendhttp.Client,
	transport *http.Transport,
	cloudHarvester cloud.Harvester,
	fpHarvester fingerprint.Harvester,
	notificationHandler *ctl.NotificationHandlerWithCancellation,
) (*Agent, error) {
	a := &Agent{
		Context:             ctx,
		debugProvide:        debug.ProvideFn,
		userAgent:           userAgent,
		store:               s,
		lastSubmissionStore: lastInventoryStore,
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
	a.inventories = map[string]*inventory{}

	// Make sure the network is working before continuing with identity
	if err := checkCollectorConnectivity(ctx.Ctx, cfg, backoff.NewRetrier(), a.userAgent, a.Context.agentKey, transport); err != nil {
		alog.WithError(err).Error("network is not available")
		return nil, err
	}

	if err := a.setAgentKey(idLookupTable); err != nil {
		return nil, fmt.Errorf("could not determine any suitable identification for this agent. Attempts to gather the hostname, cloud ID, or configured alias all failed")
	}
	llog := alog.WithField("id", a.Context.AgentIdentifier())
	llog.Debug("Bootstrap Entity Key.")

	// Create the external directory for user-generated json
	if err := disk.MkdirAll(a.extDir, 0755); err != nil {
		llog.WithField("path", a.extDir).WithError(err).Error("External json directory could not be initialized")
		return nil, err
	}

	// Create input channel for plugins to feed data back to the agent
	a.Context.ch = make(chan PluginOutput)
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

func (i IDLookup) getAgentKey() (agentKey string, err error) {
	if len(i) == 0 {
		err = fmt.Errorf("No identifiers given")
		return
	}

	for _, keyType := range sysinfo.HOST_ID_TYPES {
		// Skip blank identifiers which may have found their way into the map.
		// (Specifically, Azure can sometimes give us a blank VMID - See MTBLS-1429)
		if key, ok := i[keyType]; ok && key != "" {
			return key, nil
		}
	}

	err = ErrUndefinedLookupType
	return
}

// AgentShortEntityName is the agent entity name, but without having long-hostname into account.
// It is taken from the first field in the priority.
func (i IDLookup) AgentShortEntityName() (string, error) {
	priorities := []string{
		sysinfo.HOST_SOURCE_INSTANCE_ID,
		sysinfo.HOST_SOURCE_AZURE_VM_ID,
		sysinfo.HOST_SOURCE_GCP_VM_ID,
		sysinfo.HOST_SOURCE_ALIBABA_VM_ID,
		sysinfo.HOST_SOURCE_DISPLAY_NAME,
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT,
	}

	for _, k := range priorities {
		if name, ok := i[k]; ok && name != "" {
			return name, nil
		}
	}

	return "", ErrUndefinedLookupType
}

// NewIdLookup creates a new agent ID lookup table.
func NewIdLookup(resolver hostname.Resolver, cloudHarvester cloud.Harvester, displayName string) IDLookup {
	idLookupTable := make(IDLookup)
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
func (a *Agent) registerEntityInventory(entityKey string) error {
	alog.WithField("entityKey", entityKey).Debug("Registering inventory for entity.")
	var inv inventory

	var err error
	if a.Context.cfg.RegisterEnabled {
		inv.sender, err = newPatchSenderVortex(entityKey, a.Context.agentKey, a.Context, a.store, a.userAgent, a.Context.AgentIdentity, a.provideIDs, a.entityMap, a.httpClient)
	} else {
		inv.sender, err = newPatchSender(entityKey, a.Context, a.store, a.lastSubmissionStore, a.userAgent, a.Context.AgentIdentity, a.httpClient)
	}
	if err != nil {
		return err
	}

	inv.reaper = newPatchReaper(entityKey, a.store)
	a.inventories[entityKey] = &inv

	return nil
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
	for _, plugin := range a.plugins {
		switch p := plugin.(type) {
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
func (a *Agent) storePluginOutput(plugin PluginOutput) error {

	if plugin.Data == nil {
		plugin.Data = make(PluginInventoryDataset, 0)
	}

	sort.Sort(plugin.Data)

	// Filter out ignored inventory data before writing the file out
	var sortKey string
	ignore := a.Context.Config().IgnoredInventoryPathsMap
	simplifiedPluginData := make(map[string]interface{})
DataLoop:
	for _, data := range plugin.Data {
		if data == nil {
			continue
		}
		sortKey = data.SortKey()
		pluginSource := fmt.Sprintf("%s/%s", plugin.Id, sortKey)
		if _, ok := ignore[strings.ToLower(pluginSource)]; ok {
			continue DataLoop
		}
		simplifiedPluginData[sortKey] = data
	}

	return a.store.SavePluginSource(
		plugin.EntityKey,
		plugin.Id.Category,
		plugin.Id.Term,
		simplifiedPluginData,
	)
}

// startPlugins takes all the registered plugins and starts them up serially
// we don't return until all plugins have been started
func (a *Agent) startPlugins() {
	// iterate over and start each plugin
	for _, plugin := range a.plugins {
		plugin.LogInfo()
		func(p Plugin) {
			go p.Run()
		}(plugin)
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

func (a *Agent) updateIDLookupTable(hostAliases PluginInventoryDataset) (err error) {
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
func (a *Agent) setAgentKey(idLookupTable IDLookup) error {
	key, err := idLookupTable.getAgentKey()
	if err != nil {
		return err
	}

	alog.WithField("old", a.Context.agentKey).WithField("new", key).Debug("Updating identity.")

	a.Context.agentKey = key

	if a.store != nil {
		a.store.ChangeDefaultEntity(key)
	}

	return nil
}

// Run is the main event loop for the agent it starts up the plugins
// kicks off a filesystem seed and watcher and listens for data from
// the plugins
func (a *Agent) Run() (err error) {
	alog.Info("Starting up agent...")
	// start listening for ipc messages
	_ = a.notificationHandler.Start()

	cfg := a.Context.cfg

	// Start CPU profiling
	if cfg.CPUProfile != "" {
		clog := alog.WithField("cpuProfile", cfg.CPUProfile)
		clog.Debug("Starting CPU profiling.")
		f, err := os.Create(cfg.CPUProfile)
		if err != nil {
			clog.WithError(err).Error("could not create CPU profile file")
		} else {
			defer helpers.CloseQuietly(f)
			if err := pprof.StartCPUProfile(f); err != nil {
				clog.WithError(err).Error("could not start CPU profile")
			}
			defer pprof.StopCPUProfile()
		}
	}
	// Start memory profiling
	if cfg.MemProfile != "" {
		mlog := alog.WithField("memProfile", cfg.MemProfile)
		mlog.Debug("Starting memory profiling.")
		f, err := os.Create(cfg.MemProfile)
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

	if cfg.ConnectEnabled {
		go a.connect()
	}

	alog.Debug("Starting Plugins.")
	a.startPlugins()

	if err != nil {
		alog.WithError(err).Error("failed to start troubleshooting handler")
	}

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

	// State variables
	var readyToReap bool          // Do we need to execute a reap phase?
	var sendErrorCount uint32 = 0 // Send error counter

	// Timers
	reapTimer := time.NewTicker(cfg.FirstReapInterval)
	sendTimer := time.NewTimer(cfg.SendInterval) // Send any deltas every X seconds
	debugTimer := time.Tick(time.Duration(a.Context.Config().DebugLogSec) * time.Second)

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
	if _, ok := a.inventories[a.Context.AgentIdentifier()]; !ok {
		_ = a.registerEntityInventory(a.Context.AgentIdentifier())
	}

	exit := make(chan struct{})

	go func() {
		<-a.Context.Ctx.Done()
		log.Info("Gracefully Exiting")

		if sendTimer != nil {
			sendTimer.Stop()
		}
		if reapTimer != nil {
			reapTimer.Stop()
		}
		if removeEntitiesTicker != nil {
			removeEntitiesTicker.Stop()
		}
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

		if a.notificationHandler != nil {
			a.notificationHandler.Stop()
		}

		close(exit)

		// Should not reach here, just a guard.
		//<-time.After(service.GracefulExitTimeout)
		//log.Warn("graceful stop time exceeded... forcing stop")
		//os.Exit(0)
	}()

	// three states
	//  -- reading data to write to json
	//  -- reaping
	//  -- sending
	// ready to consume events
	for {
		select {
		case <-exit:
			return nil
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

				if _, ok := a.inventories[data.EntityKey]; !ok {
					_ = a.registerEntityInventory(data.EntityKey)
				}

				if !data.NotApplicable {
					if err := a.storePluginOutput(data); err != nil {
						alog.WithError(err).Error("problem storing plugin output")
					}
					a.inventories[data.EntityKey].needsReaping = true
				}
			}
		case <-reapTimer.C:
			{
				for _, inventory := range a.inventories {
					if !readyToReap {
						if len(distinctPlugins) <= len(idsReporting) {
							alog.Debug("Signalling initial reap.")
							readyToReap = true
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
					if readyToReap && inventory.needsReaping {
						reapTimer.Stop()
						reapTimer = time.NewTicker(cfg.ReapInterval)
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
			if !readyToReap {
				alog.Debug("Maximum initial reap delay exceeded - marking inventory as ready to report.")
				readyToReap = true
				for _, inventory := range a.inventories {
					inventory.needsCleanup = true
				}
			}
		case <-sendTimer.C:
			{
				backoffMax := config.MAX_BACKOFF
				for _, inventory := range a.inventories {
					err := inventory.sender.Process()
					if err != nil {
						if ingestError, ok := err.(*inventoryapi.IngestError); ok {
							if ingestError.StatusCode == http.StatusTooManyRequests {
								alog.Warn("server is rate limiting inventory for this Infrastructure Agent")
								backoffMax = config.RATE_LIMITED_BACKOFF
								sendErrorCount = helpers.MaxBackoffErrorCount
							}
						} else {
							sendErrorCount++
						}
						alog.WithError(err).WithField("errorCount", sendErrorCount).
							Debug("Inventory sender can't process after retrying.")
						break // Assuming break will try to send later the data from the missing inventory senders
					} else {
						sendErrorCount = 0
					}
				}
				sendTimerVal := helpers.ExpBackoff(cfg.SendInterval,
					time.Duration(backoffMax)*time.Second,
					sendErrorCount)
				sendTimer.Reset(sendTimerVal)
			}
		case <-debugTimer:
			{
				debugInfo, err := a.debugProvide()
				if err != nil {
					alog.WithError(err).Error("debug error")
				} else if debugInfo != "" {
					alog.Debug(debugInfo)
				}
			}
		case <-removeEntitiesTicker.C:
			pastPeriodReportedEntities := reportedEntities
			reportedEntities = map[string]bool{} // reset the set of reporting entities the next period
			alog.Debug("Triggered periodic removal of outdated entities.")
			a.removeOutdatedEntities(pastPeriodReportedEntities)
		}
	}
}

func (a *Agent) removeOutdatedEntities(reportedEntities map[string]bool) {
	alog.Debug("Triggered periodic removal of outdated entities.")
	// The entities to remove are those entities that haven't reported activity in the last period and
	// are registered in the system
	entitiesToRemove := map[string]bool{}
	for entityKey := range a.inventories {
		entitiesToRemove[entityKey] = true
	}
	delete(entitiesToRemove, a.Context.AgentIdentifier()) // never delete local entity
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

func (c *context) SendData(data PluginOutput) {
	c.ch <- data
}

func (c *context) ActiveEntitiesChannel() chan string {
	return c.activeEntities
}

func (c *context) SendEvent(event sample.Event, entityKey entity.Key) {
	if c.eventSender != nil {
		// limits any string field larger than 4095 chars
		if c.cfg.TruncTextValues {
			event = metric.TruncateLength(event, metric.NRDBLimit)
		}

		includeSample := c.shouldIncludeEvent(event)
		if includeSample {
			if err := c.eventSender.QueueEvent(event, entityKey); err != nil {
				alog.WithField(
					"entityKey", entityKey,
				).WithError(err).Error("could not queue event")
			}
		}
	}
}

func (c *context) Unregister(id ids.PluginID) {
	c.ch <- NewNotApplicableOutput(id)
}

func (c *context) Config() *config.Config {
	return c.cfg
}

func (c *context) AgentIdentifier() string {
	return c.agentKey
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
	aclog := log.WithComponent("AgentContext")
	aclog.Debug("Invoking Run() on all the plugins registered for reconnection.")
	c.reconnecting.Range(triggerAddReconnecting(aclog))
}

// triggerAddReconnecting is used with sync.Map.Range to iterate through all plugins and reconnect them
func triggerAddReconnecting(aclog log.Entry) func(pluginID interface{}, plugin interface{}) bool {
	return func(pluginID, plugin interface{}) bool {
		aclog.WithField("plugin", pluginID).Debug("Reconnecting plugin.")
		func(p Plugin) {
			go p.Run()
		}(plugin.(Plugin))
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

func (a *Agent) connect() {
	alog.Debug("Performing connect.")
	a.Context.SetAgentIdentity(a.connectSrv.Connect())

	updateFreq := time.Duration(a.Context.cfg.FingerprintUpdateFreqSec) * time.Second
	ticker := time.NewTicker(updateFreq)

	for range ticker.C {
		alog.Debug("Performing connect update.")
		identity, err := a.connectSrv.ConnectUpdate(a.Context.AgentIdentity())
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
