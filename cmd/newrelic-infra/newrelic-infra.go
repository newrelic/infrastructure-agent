// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:generate goversioninfo

package main

import (
	context2 "context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/cmd/newrelic-infra/dnschecks"
	"github.com/newrelic/infrastructure-agent/cmd/newrelic-infra/initialize"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	ccBackoff "github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/backoff"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/runintegration"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/service"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/stopintegration"
	selfInstrumentation "github.com/newrelic/infrastructure-agent/internal/agent/instrumentation"
	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/httpapi"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/files"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/v3legacy"
	"github.com/newrelic/infrastructure-agent/internal/socketapi"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"github.com/newrelic/infrastructure-agent/pkg/fs/systemd"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/recover"
	http2 "github.com/newrelic/infrastructure-agent/pkg/http"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track"
	v4 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4"
	integrationsConfig "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
	wlog "github.com/newrelic/infrastructure-agent/pkg/log"
	logFilter "github.com/newrelic/infrastructure-agent/pkg/log/filter"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

var (
	dryRun bool

	// Specifies the path to look for integrations config files when running in dry-run mode.
	integrationConfigPath string

	configFile  string
	validate    bool
	showVersion bool
	debug       bool
	cpuprofile  string
	memprofile  string
	// v3tov4       string # v3tov4 disabled.
	verbose      int
	startTime    time.Time
	buildVersion = "development"
	gitCommit    = ""
	svcName      = "newrelic-infra"
	buildDate    = ""
)

func elapsedTime() time.Duration {
	return time.Since(startTime)
}

func init() {
	flag.BoolVar(&dryRun, "dry_run", false, "Run the NR Infrastructure agent in dry_run mode.")

	flag.StringVar(&integrationConfigPath, "integration_config_path", "", "Path of the newrelic integrations configuration files when running in dry-run mode. Can be a file or a directory. (Default: plugin_dir)")
	flag.StringVar(&configFile, "config", "", "Overrides default configuration file")
	flag.BoolVar(&validate, "validate", false, "Validate agent config and exit")
	flag.BoolVar(&showVersion, "version", false, "Shows version details")
	flag.BoolVar(&debug, "debug", false, "Enables agent debugging functionality")
	flag.StringVar(&cpuprofile, "cpuprofile", "", "Writes cpu profile to `file`")
	flag.StringVar(&memprofile, "memprofile", "", "Writes memory profile to `file`")
	// flag.StringVar(&v3tov4, "v3tov4", "", "Converts v3 config into v4. v3tov4=/path/to/config:/path/to/definition:/path/to/output:overwrite")

	flag.IntVar(&verbose, "verbose", 0, "Higher numbers increase levels of logging. When enabled overrides provided config.")
}

var alog = wlog.WithComponent("New Relic Infrastructure Agent")

func main() {
	flag.Parse()

	defer recover.PanicHandler(recover.LogAndFail)

	startTime = time.Now()

	memLog := wlog.NewMemLogger(os.Stdout)
	wlog.SetOutput(memLog)

	if showVersion {
		fmt.Printf("New Relic Infrastructure Agent version: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
			buildVersion, runtime.Version(), gitCommit, buildDate)
		os.Exit(0)
	}

	//if v3tov4 != "" {
	//
	//	v3tov4Args := strings.Split(v3tov4, ":")
	//
	//	if len(v3tov4Args) != 4 {
	//		fmt.Printf("v3tov4 argument should contain 4 parts")
	//		os.Exit(1)
	//	}
	//
	//	err := integrationsConfig.MigrateV3toV4(v3tov4Args[0], v3tov4Args[1], v3tov4Args[2], v3tov4Args[3] == "true")
	//	if err != nil {
	//		fmt.Println(err)
	//		os.Exit(1)
	//	}
	//
	//	fmt.Printf("The config has been migrated and placed in: %s", v3tov4Args[2])
	//
	//	// rename old files to .bk
	//	err = os.Rename(v3tov4Args[0], v3tov4Args[0]+".bk")
	//
	//	if err != nil {
	//		fmt.Println(err)
	//		os.Exit(1)
	//	}
	//
	//	err = os.Rename(v3tov4Args[1], v3tov4Args[1]+".bk")
	//
	//	if err != nil {
	//		fmt.Println(err)
	//		os.Exit(1)
	//	}
	//
	//	os.Exit(0)
	//}

	timedLog := alog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"version":     buildVersion,
			"elapsedTime": elapsedTime(),
		}
	})

	timedLog.Debug("Configuring handlers.")

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGQUIT)
		for {
			<-sigs
			buf := make([]byte, 1<<20)
			runtime.Stack(buf, true)
			alog.Info(fmt.Sprintf("== SIGQUIT RECEIVED ==\n** goroutine dump begin **\n%s\n** goroutine dump end **", buf))
		}
	}()

	timedLog.Debug("Loading configuration.")

	cfg, err := config.LoadConfig(configFile)

	if validate {
		if err != nil {
			alog.Info(fmt.Sprintf("config validation failed with error: %s", err.Error()))
			os.Exit(0)
		}
		alog.Info("config validation finished without errors")
		os.Exit(0)
	}

	if err != nil {
		alog.WithError(err).Error("can't load configuration file")
		os.Exit(1)
	}

	if dryRun {
		executeIntegrationsDryRunMode(integrationConfigPath, cfg)
		os.Exit(0)
	}

	// override YAML with CLI flags
	if verbose > config.NonVerboseLogging {
		cfg.Verbose = verbose
	}
	if cpuprofile != "" {
		cfg.CPUProfile = cpuprofile
	}
	if memprofile != "" {
		cfg.MemProfile = memprofile
	}

	if cfg.Log.IsSmartLogging() {
		wlog.EnableSmartVerboseMode(cfg.Log.GetSmartLogLevelLimit())
	}

	if debug || cfg.WebProfile {
		alog.Info("starting pprof server at http://localhost:6060")
		go recover.FuncWithPanicHandler(recover.LogAndContinue, func() {
			alog.WithError(http.ListenAndServe("localhost:6060", nil)).Warn("trying to open a connection in :6060")
		})
	}

	configureLogFormat(cfg.Log)

	// Send logging where it's supposed to go.
	agentLogsToFile := configureLogRedirection(&cfg.Log, memLog)

	// Runtime config setup.
	troubleCfg := config.NewTroubleshootCfg(cfg.Log.IsTroubleshootMode(), agentLogsToFile, cfg.GetLogFile())
	logFwCfg := config.NewLogForward(cfg, troubleCfg)

	// If parsedConfig.MaxProcs < 1, leave GOMAXPROCS to its previous value,
	// which, if not set by the environment, is the number of processors that
	// have been detected by the system.
	// Note that if the `max_procs` option is unset, default value for
	// parsedConfig.MaxProcs is 1.
	runtime.GOMAXPROCS(cfg.MaxProcs)

	logConfig(cfg)

	err = initialize.OsProcess(cfg)
	if err != nil {
		alog.WithError(err).Error("Performing OS-specific process initialization...")
		os.Exit(1)
	}

	err = initializeAgentAndRun(cfg, logFwCfg)
	if err != nil {
		timedLog.WithError(err).Error("Agent run returned an error.")
		os.Exit(1)
	}
}

func logConfig(c *config.Config) {
	// Log the configuration.
	c.LogInfo()

	// Runtime evaluated.
	alog.WithFieldsF(func() logrus.Fields {
		fields := logrus.Fields{
			"pluginDir":      c.PluginInstanceDirs,
			"maxProcs":       runtime.GOMAXPROCS(-1),
			"agentUser":      c.AgentUser,
			"executablePath": c.ExecutablePath,
		}
		if wlog.IsLevelEnabled(logrus.DebugLevel) {
			fields["identityURL"] = c.IdentityURL
		}
		return fields
	}).Info("runtime configuration")
}

var aslog = wlog.WithComponent("AgentService").WithFields(logrus.Fields{
	"service": svcName,
})

func initializeAgentAndRun(c *config.Config, logFwCfg config.LogForward) error {
	pluginSourceDirs := getPluginSourceDirs(c)

	v4ManagerConfig := v4.NewManagerConfig(
		c.Log.VerboseEnabled(),
		c.DefaultIntegrationsTempDir,
		c.Features,
		c.PassthroughEnvironment,
		c.PluginInstanceDirs,
		pluginSourceDirs,
	)

	userAgent := agent.GenerateUserAgent("New Relic Infrastructure Agent", buildVersion)
	transport := backendhttp.BuildTransport(c, backendhttp.ClientTimeout)
	transport = backendhttp.NewRequestDecoratorTransport(c, transport)

	httpClient := backendhttp.GetHttpClient(backendhttp.ClientTimeout, transport)

	cmdChannelURL := strings.TrimSuffix(c.CommandChannelURL, "/")
	ccSvcURL := fmt.Sprintf("%s%s", cmdChannelURL, c.CommandChannelEndpoint)
	caClient := commandapi.NewClient(ccSvcURL, c.License, userAgent, httpClient.Do)
	ffManager := feature_flags.NewManager(c.Features)
	il := newInstancesLookup(v4ManagerConfig)

	fatal := func(err error, message string) {
		aslog.WithError(err).Error(message)
		os.Exit(1)
	}

	aslog.Info("Checking network connectivity...")

	if c.Log.HasIncludeFilter(config.TracesFieldComponent, config.HttpTracer) {
		err := dnschecks.RunChecks(c.CollectorURL, c.StartupConnectionTimeout, transport, aslog)
		if err != nil {
			os.Exit(1)
		}
	}

	err := waitForNetwork(c.CollectorURL, c.StartupConnectionTimeout, c.StartupConnectionRetries, transport)
	if err != nil {
		fatal(err, "Can't reach the New Relic collector.")
	}

	timedLog := aslog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"elapsedTime": elapsedTime(),
		}
	})

	// Basic initialization of the agent.
	timedLog.WithField("version", buildVersion).Info("Initializing")

	registerClient, err := identityapi.NewRegisterClient(
		c.IdentityURL,
		c.License,
		userAgent,
		c.PayloadCompressionLevel,
		httpClient,
	)
	if err != nil {
		return err
	}

	agt, err := agent.NewAgent(
		c,
		buildVersion,
		userAgent,
		ffManager)
	if err != nil {
		fatal(err, "Agent cannot initialize.")
	}

	selfInstrumentation.InitSelfInstrumentation(c, agt.Context.HostnameResolver())

	defer agt.Terminate()

	if err := initialize.AgentService(c); err != nil {
		fatal(err, "Can't complete platform specific initialization.")
	}

	metricsSenderConfig := dm.NewConfig(c.DMIngestURL(), c.Fedramp, c.License, time.Duration(c.DMSubmissionPeriod)*time.Second, c.MaxMetricBatchEntitiesCount, c.MaxMetricBatchEntitiesQueue)
	dmSender, err := dm.NewDMSender(metricsSenderConfig, transport, agt.Context.IdContext().AgentIdentity)
	if err != nil {
		return err
	}

	// queues integration run requests
	definitionQ := make(chan integration.Definition, 100)
	// queues config entries requests
	configEntryQ := make(chan configrequest.Entry, 100)

	dmEmitter := dm.NewEmitter(agt.GetContext(), dmSender, registerClient, ffManager)

	// track stoppable integrations
	tracker := track.NewTracker(dmEmitter)

	integrationEmitter := emitter.NewIntegrationEmittor(agt, dmEmitter, ffManager)
	cfgLoader := integrationsConfig.NewPathLoader()
	integrationManager := v4.NewManager(
		v4ManagerConfig,
		cfgLoader,
		integrationEmitter,
		il,
		definitionQ,
		configEntryQ,
		tracker,
		agt.Context.IDLookup(),
	)

	// Command channel handlers
	backoffSecsC := make(chan int, 1) // 1 won't block on initial cmd-channel fetch
	boHandler := ccBackoff.NewHandler(backoffSecsC)
	ffHandle := fflag.NewHandler(c, ffManager, wlog.WithComponent("FFHandler"))
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", ffHandle.Handle)
	riHandler := runintegration.NewHandler(definitionQ, il, dmEmitter, wlog.WithComponent("runintegration.Handler"))
	siHandler := stopintegration.NewHandler(tracker, il, dmEmitter, wlog.WithComponent("stopintegration.Handler"))
	// Command channel service
	ccService := service.NewService(
		caClient,
		c.CommandChannelIntervalSec,
		backoffSecsC,
		boHandler,
		ffHandler,
		riHandler,
		siHandler,
	)
	initCmdResponse, err := ccService.InitialFetch(agt.Context.Ctx)
	if err != nil {
		aslog.WithError(err).Warn("Commands initial fetch failed.")
	}

	// Initialise the agent after fetching FF.
	agt.Init()

	if c.StatusServerEnabled || c.HTTPServerEnabled {
		rlog := wlog.WithComponent("status.Reporter")
		timeoutD, err := time.ParseDuration(c.StartupConnectionTimeout)
		if err != nil {
			// This should never happen, as the correct format is checked during NormalizeConfig.
			aslog.WithError(err).Error("invalid startup_connection_timeout value, cannot run status server")
		} else {
			rep := status.NewReporter(agt.Context.Ctx, rlog, c.StatusEndpoints, timeoutD, transport, agt.Context.AgentIdnOrEmpty, agt.Context.EntityKey, c.License, userAgent)

			apiSrv, err := httpapi.NewServer(rep, integrationEmitter)
			if c.HTTPServerEnabled {
				apiSrv.Ingest.Enable(c.HTTPServerHost, c.HTTPServerPort)
			}

			if c.HTTPServerCert != "" && c.HTTPServerKey != "" {
				apiSrv.Ingest.TLS(c.HTTPServerCert, c.HTTPServerKey)
			}

			if c.HTTPServerCA != "" {
				apiSrv.Ingest.VerifyTLSClient(c.HTTPServerCA)
			}

			if c.StatusServerEnabled {
				apiSrv.Status.Enable("localhost", c.StatusServerPort)
			}

			if err != nil {
				aslog.WithError(err).Error("cannot run api server")
			} else {
				go apiSrv.Serve(agt.Context.Ctx)
			}
		}
	}

	if c.TCPServerEnabled {
		go socketapi.NewServer(integrationEmitter, c.TCPServerPort).Serve(agt.Context.Ctx)
	}

	// Start all plugins we want the agent to run.
	if err = plugins.RegisterPlugins(agt); err != nil {
		aslog.WithError(err).Error("fatal error while registering plugins")
		os.Exit(1)
	}

	fbVerbose := c.Log.Level == config.LogLevelTrace && c.Log.HasIncludeFilter(config.TracesFieldName, config.SupervisorTrace)
	confTempFolder := filepath.Join(c.AgentTempDir, v4.FbConfTempFolderNameDefault)
	fbIntCfg := v4.NewFBSupervisorConfig(
		ffManager,
		c.AgentDir,
		config.DefaultIntegrationsDir,
		c.LoggingBinDir,
		c.FluentBitExePath,
		c.FluentBitNRLibPath,
		c.FluentBitParsersPath,
		fbVerbose,
		confTempFolder,
	)

	if fbIntCfg.IsLogForwarderAvailable() {
		logCfgLoader := logs.NewFolderLoader(logFwCfg, agt.Context.Identity, agt.Context.HostnameResolver())
		logSupervisor := v4.NewFBSupervisor(
			fbIntCfg,
			logCfgLoader,
			agt.Context.AgentIDUpdateNotifier(),
			agt.Context.HostnameChangeNotifier(),
			agt.Context.SendEvent,
		)
		ffHandle.SetFBRestarter(logSupervisor)
		go logSupervisor.Run(agt.Context.Ctx)
	} else {
		aslog.Debug("Log forwarder is not available for this platform. The agent will start without log forwarding support.")
	}

	ffHandle.SetOHIHandler(integrationManager)

	go integrationManager.Start(agt.Context.Ctx)

	go ccService.Run(agt.Context.Ctx, agt.Context.AgentIdnOrEmpty, initCmdResponse)

	pluginRegistry := legacy.NewPluginRegistry(pluginSourceDirs, c.PluginInstanceDirs)
	if err := pluginRegistry.LoadPlugins(); err != nil {
		fatal(err, "Can't load plugins.")
	}
	pluginConfig, err := legacy.LoadPluginConfig(pluginRegistry, c.PluginConfigFiles)
	if err != nil {
		fatal(err, "Can't load plugin configuration.")
	}
	runner := legacy.NewPluginRunner(pluginRegistry, agt)
	for _, pluginConf := range pluginConfig.PluginConfigs {
		if err := runner.ConfigurePlugin(pluginConf, agt.Context.ActiveEntitiesChannel()); err != nil {
			fatal(err, fmt.Sprint("Can't configure plugin.", pluginConf))
		}
	}

	if err := runner.ConfigureV1Plugins(agt.Context); err != nil {
		aslog.WithError(err).Debug("Can't configure integrations.")
	}

	timedLog.Info("New Relic infrastructure agent is running.")

	return agt.Run()
}

// newInstancesLookup creates an instance lookup that:
// - looks in the v3 legacy definitions repository for defined commands
// - looks in the definition folders (and bin/ subfolders) for executable names
func newInstancesLookup(cfg v4.ManagerConfig) integration.InstancesLookup {
	const executablesSubFolder = "bin"

	var execFolders []string
	for _, df := range cfg.DefinitionFolders {
		execFolders = append(execFolders, df)
		execFolders = append(execFolders, filepath.Join(df, executablesSubFolder))
	}
	legacyDefinedCommands := v3legacy.NewDefinitionsRepo(v3legacy.LegacyConfig{
		DefinitionFolders: cfg.DefinitionFolders,
		Verbose:           cfg.Verbose,
		TempDir:           cfg.TempDir,
	})
	return integration.InstancesLookup{
		Legacy: legacyDefinedCommands.NewDefinitionCommand,
		ByName: files.Executables{Folders: execFolders}.Path,
	}
}

// configureLogFormat checks the config and sets the log format accordingly.
func configureLogFormat(cfg config.LogConfig) {
	// get default logrus formatter
	var formatter logrus.Formatter = wlog.GetFormatter()
	if cfg.Format == config.LogFormatJSON {
		jsonFormatter := &logrus.JSONFormatter{
			DataKey: "context",

			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime: "timestamp",
			},
		}
		formatter = jsonFormatter
	}
	// Apply filters to agent logs. Filters are only available in the log configuration object.
	logFilterCfg := logFilter.FilteringFormatterConfig{
		IncludeFilters: cfg.IncludeFilters,
		ExcludeFilters: cfg.ExcludeFilters,
	}

	formatter = logFilter.NewFilteringFormatter(logFilterCfg, formatter)

	wlog.SetFormatter(formatter)
}

// Either route standard logging to stdout (for Linux, so it gets copied to syslog as appropriate)
// or copy it to stdout and a log file for Mac/Windows so we don't lose the logging when running
// as a service.
func configureLogRedirection(config *config.LogConfig, memLog *wlog.MemLogger) (onFile bool) {
	if config.File == "" && !(config.IsTroubleshootMode() && systemd.IsAgentRunningOnSystemD()) {
		wlog.SetOutput(os.Stdout)
		return
	}

	// Redirect all output to both stdout and the agent's own log file.
	logWriter, err := newLogWriter(config)
	if err != nil {
		alog.WithField("action", "configureLogRedirection").WithError(err).Error("Can't open log file.")
		os.Exit(1)
	}

	alog.WithFields(logrus.Fields{
		"action":      "configureLogRedirection",
		"logFile":     config.File,
		"logToStdout": config.IsStdoutEnabled(),
	}).Debug("Redirecting output to a file.")

	// Write all previous logs, which are stored in memLog, to the file.
	_, err = memLog.WriteBuffer(logWriter)
	if err != nil {
		wlog.WithError(err).Debug("Failed writing log to file.")
	} else {
		onFile = true
	}
	wlog.SetOutput(wlog.NewStdoutTeeLogger(logWriter, config.IsStdoutEnabled()))
	return
}

// newLogWriter returns an io.Writer to be used by the logger as an output.
func newLogWriter(config *config.LogConfig) (io.Writer, error) {
	logRotateConfig := config.Rotate
	if !logRotateConfig.IsSet() || !logRotateConfig.IsEnabled() {
		return disk.OpenFile(config.File, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	}

	rotateCfg := wlog.FileWithRotationConfig{
		File:            config.File,
		FileNamePattern: logRotateConfig.FilePattern,
		MaxSizeInBytes:  int64(*logRotateConfig.MaxSizeMb) << 20,
		MaxFiles:        logRotateConfig.MaxFiles,
		Compress:        logRotateConfig.CompressionEnabled,
	}
	return wlog.NewFileWithRotation(rotateCfg).Open()
}

// waitForNetwork verifies that there is network connectivity to the collector
// endpoint, or waits until it is available.
// It differs from the agent.checkCollectorConnectivity function in that the
// agent is not identified: We just verify that the network is available.
// If we don't wait for the network, it may happen that a cloud instance doesn't
// properly get the cloud metadata during the initial samples, and different
// entity IDs are seen for some minutes after the cloud instance is restarted.
func waitForNetwork(collectorURL, timeout string, retries int, transport http.RoundTripper) (err error) {
	if collectorURL == "" {
		return
	}

	retrier := backoff.NewRetrier()

	// If StartupConnectionRetries is negative, we keep checking the connection
	// until it succeeds.
	timeoutD, err := time.ParseDuration(timeout)
	if err != nil {
		// This should never happen, as the correct format is checked
		// during NormalizeConfig.
		return
	}
	var timedout bool

	for {
		timedout, err = checkEndpointReachable(collectorURL, timeoutD, transport)
		if timedout {
			if retries >= 0 {
				retries -= 1
				if retries <= 0 {
					break
				}
			}
			aslog.WithError(err).WithField("collector_url", collectorURL).
				Warn("Collector endpoint not reachable, retrying...")
			retrier.SetNextRetryWithBackoff()
			time.Sleep(retrier.RetryAfter())
		} else {
			// Otherwise we got a response, so break out.
			break
		}
	}
	return
}

func checkEndpointReachable(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
) (timedOut bool, err error) {
	var request *http.Request
	if request, err = http.NewRequest("HEAD", collectorURL, nil); err != nil {
		return false, fmt.Errorf("unable to prepare reachability request: %v, error: %s", request, err)
	}

	if wlog.IsLevelEnabled(logrus.TraceLevel) {
		request = http2.WithTracer(request, "checkEndpointReachable")
	}
	client := backendhttp.GetHttpClient(timeout, transport)
	if _, err = client.Do(request); err != nil {
		if e2, ok := err.(net.Error); ok && (e2.Timeout() || e2.Temporary()) {
			timedOut = true
		}
		if errURL, ok := err.(*url.Error); ok {
			aslog.WithError(errURL).Warn("URL error detected. May be a configuration problem or a network connectivity issue.")
			timedOut = true
		}
	}

	return
}

func getPluginSourceDirs(ac *config.Config) []string {
	pluginSourceDirs := []string{
		filepath.Join(ac.SafeBinDir, config.DefaultIntegrationsDir),
		filepath.Join(ac.SafeBinDir, "custom-integrations"),
		ac.CustomPluginInstallationDir,
		filepath.Join(ac.AgentDir, "custom-integrations"),
		filepath.Join(ac.AgentDir, config.DefaultIntegrationsDir),
		filepath.Join(ac.AgentDir, "bundled-plugins"),
		filepath.Join(ac.AgentDir, "plugins"),
	}
	return helpers.RemoveEmptyAndDuplicateEntries(pluginSourceDirs)
}

// executeIntegrationsDryRunMode is used for dry-run mode. It will read the integration config files,
// execute all the integrations and print the output to stdout.
func executeIntegrationsDryRunMode(configPath string, ac *config.Config) {
	// Config path can be a directory with multiple integration config files or a specific config file.
	// If configPath is not specified we use the default locations for the nr integration configs.
	integrationConfigPaths := ac.PluginInstanceDirs
	if configPath != "" {
		integrationConfigPaths = []string{configPath}
	}

	v4ManagerConfig := v4.NewManagerConfig(
		ac.Log.VerboseEnabled(),
		ac.DefaultIntegrationsTempDir,
		ac.Features,
		ac.PassthroughEnvironment,
		integrationConfigPaths,
		getPluginSourceDirs(ac),
	)

	var definitionQ chan integration.Definition
	var configEntryQ chan configrequest.Entry
	var tracker *track.Tracker

	hostnameResolver := hostname.CreateResolver(
		ac.OverrideHostname, ac.OverrideHostnameShort, ac.DnsHostnameResolution)

	// Initialize the cloudDetector.
	cloudHarvester := cloud.NewDetector(ac.DisableCloudMetadata, ac.CloudMaxRetryCount, ac.CloudRetryBackOffSec, ac.CloudMetadataExpiryInSec, ac.CloudMetadataDisableKeepAlive)
	cloudHarvester.Initialize()

	agentIDLookup := agent.NewIdLookup(hostnameResolver, cloudHarvester, ac.DisplayName)

	pluginRegistry := legacy.NewPluginRegistry(v4ManagerConfig.DefinitionFolders, ac.PluginInstanceDirs)
	if err := pluginRegistry.LoadPlugins(); err != nil {
		aslog.WithError(err).Error("Can't load plugins.")
		os.Exit(1)
	}

	integrationEmitter := emitter.NewStdoutEmitter()

	cfgLoader := integrationsConfig.NewPathLoader()
	integrationManager := v4.NewManager(
		v4ManagerConfig,
		cfgLoader,
		integrationEmitter,
		newInstancesLookup(v4ManagerConfig),
		definitionQ,
		configEntryQ,
		tracker,
		agentIDLookup,
	)
	integrationManager.RunOnce(context2.Background())
}
