// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:generate goversioninfo

package main

import (
	context2 "context"
	"flag"
	"fmt"
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

	v3 "github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v3"
	v3config "github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v3/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/logs"
	dm2 "github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/outputhandler/v4/emitter"

	selfInstrumentation "github.com/newrelic/infrastructure-agent/internal/agent/instrumentation"
	"github.com/newrelic/infrastructure-agent/pkg/config/migrate"

	"github.com/newrelic/infrastructure-agent/internal/httpapi"
	"github.com/newrelic/infrastructure-agent/internal/instrumentation"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	ccBackoff "github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/backoff"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/runintegration"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/service"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/stopintegration"
	"github.com/newrelic/infrastructure-agent/internal/agent/status"
	"github.com/newrelic/infrastructure-agent/internal/socketapi"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/files"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/cmd/newrelic-infra/initialize"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"github.com/newrelic/infrastructure-agent/pkg/fs/systemd"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/recover"
	v4 "github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4"
	wlog "github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

var (
	configFile   string
	validate     bool
	showVersion  bool
	debug        bool
	cpuprofile   string
	memprofile   string
	v3tov4       string
	verbose      int
	startTime    time.Time
	buildVersion = "development"
	gitCommit    = ""
	svcName      = "newrelic-infra"
)

func elapsedTime() time.Duration {
	return time.Since(startTime)
}

func init() {
	flag.StringVar(&configFile, "config", "", "Overrides default configuration file")
	flag.BoolVar(&validate, "validate", false, "Validate agent config and exit")
	flag.BoolVar(&showVersion, "version", false, "Shows version details")
	flag.BoolVar(&debug, "debug", false, "Enables agent debugging functionality")
	flag.StringVar(&cpuprofile, "cpuprofile", "", "Writes cpu profile to `file`")
	flag.StringVar(&memprofile, "memprofile", "", "Writes memory profile to `file`")
	flag.StringVar(&v3tov4, "v3tov4", "", "Converts v3 config into v4. v3tov4=/path/to/config:/path/to/definition:/path/to/output:overwrite")

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
		fmt.Printf("New Relic Infrastructure Agent version: %s, GoVersion: %s, GitCommit: %s\n", buildVersion, runtime.Version(), gitCommit)
		os.Exit(0)
	}

	if v3tov4 != "" {

		v3tov4Args := strings.Split(v3tov4, ":")

		if len(v3tov4Args) != 4 {
			fmt.Printf("v3tov4 argument should contain 4 parts")
			os.Exit(1)
		}

		err := migrate.V3toV4(v3tov4Args[0], v3tov4Args[1], v3tov4Args[2], v3tov4Args[3] == "true")

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("The config has been migrated and placed in: %s", v3tov4Args[2])

		// rename old files to .bk
		err = os.Rename(v3tov4Args[0], v3tov4Args[0]+".bk")

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = os.Rename(v3tov4Args[1], v3tov4Args[1]+".bk")

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		os.Exit(0)
	}

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

	if cfg.Verbose == config.SmartVerboseLogging {
		wlog.EnableSmartVerboseMode(cfg.SmartVerboseModeEntryLimit)
	}

	if debug || cfg.WebProfile {
		alog.Info("starting pprof server at http://localhost:6060")
		go recover.FuncWithPanicHandler(recover.LogAndContinue, func() {
			alog.WithError(http.ListenAndServe("localhost:6060", nil)).Warn("trying to open a connection in :6060")
		})
	}

	configureLogFormat(cfg.LogFormat)

	// Send logging where it's supposed to go.
	agentLogsToFile := configureLogRedirection(cfg, memLog)

	trace.EnableOn(cfg.FeatureTraces)

	// Runtime config setup.
	troubleCfg := config.NewTroubleshootCfg(cfg.IsTroubleshootMode(), agentLogsToFile, cfg.GetLogFile())
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
	pluginSourceDirs := []string{
		c.CustomPluginInstallationDir,
		filepath.Join(c.AgentDir, "custom-integrations"),
		filepath.Join(c.AgentDir, config.DefaultIntegrationsDir),
		filepath.Join(c.AgentDir, "bundled-plugins"),
		filepath.Join(c.AgentDir, "plugins"),
	}
	pluginSourceDirs = helpers.RemoveEmptyAndDuplicateEntries(pluginSourceDirs)

	integrationCfg := v4.NewConfig(
		c.Verbose,
		c.Features,
		c.PassthroughEnvironment,
		c.PluginInstanceDirs,
		pluginSourceDirs,
	)

	userAgent := agent.GenerateUserAgent("New Relic Infrastructure Agent", buildVersion)
	transport := backendhttp.BuildTransport(c, backendhttp.ClientTimeout)
	httpClient := backendhttp.GetHttpClient(backendhttp.ClientTimeout, transport)
	cmdChannelURL := strings.TrimSuffix(c.CommandChannelURL, "/")
	ccSvcURL := fmt.Sprintf("%s%s", cmdChannelURL, c.CommandChannelEndpoint)
	caClient := commandapi.NewClient(ccSvcURL, c.License, userAgent, httpClient.Do)
	ffManager := feature_flags.NewManager(c.Features)
	il := newInstancesLookup(integrationCfg)

	fatal := func(err error, message string) {
		aslog.WithError(err).Error(message)
		os.Exit(1)
	}

	aslog.Info("Checking network connectivity...")
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

	instruments, err := initInstrumentation(agt.GetContext().Context(), c.AgentMetricsEndpoint)
	if err != nil {
		return fmt.Errorf("cannot initialize prometheus exporter: %v", err)
	}
	wlog.Instrument(instruments.Measure)

	metricsSenderConfig := dm2.NewConfig(c.DMIngestURL(), c.Fedramp, c.License, time.Duration(c.DMSubmissionPeriod)*time.Second, c.MaxMetricBatchEntitiesCount, c.MaxMetricBatchEntitiesQueue)
	dmSender, err := dm2.NewDMSender(metricsSenderConfig, transport, agt.Context.IdContext().AgentIdentity)
	if err != nil {
		return err
	}

	// queues integration run requests
	definitionQ := make(chan integration.Definition, 100)
	// queues config entries requests
	configEntryQ := make(chan configrequest.Entry, 100)
	// queues integration terminated definitions
	terminateDefinitionQ := make(chan string, 100)

	dmEmitter := dm2.NewEmitter(agt.GetContext(), dmSender, registerClient, instruments.Measure)

	// track stoppable integrations
	tracker := track.NewTracker(dmEmitter)

	pluginRegistry := v3config.NewPluginRegistry(pluginSourceDirs, c.PluginInstanceDirs)
	if err := pluginRegistry.LoadPlugins(); err != nil {
		fatal(err, "Can't load plugins.")
	}

	integrationEmitter := emitter.NewIntegrationEmittor(agt, dmEmitter, ffManager)
	integrationManager := v4.NewManager(
		integrationCfg,
		integrationEmitter,
		il,
		definitionQ,
		terminateDefinitionQ,
		configEntryQ,
		tracker,
		agt.Context.IDLookup(),
		pluginRegistry,
		c.License,
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

	if c.StatusServerEnabled || c.HTTPServerEnabled {
		rlog := wlog.WithComponent("status.Reporter")
		timeoutD, err := time.ParseDuration(c.StartupConnectionTimeout)
		if err != nil {
			// This should never happen, as the correct format is checked during NormalizeConfig.
			aslog.WithError(err).Error("invalid startup_connection_timeout value, cannot run status server")
		} else {
			rep := status.NewReporter(agt.Context.Ctx, rlog, c.StatusEndpoints, timeoutD, transport, agt.Context.AgentIdnOrEmpty, c.License, userAgent)

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

	// log-forwarder
	fbIntCfg := v4.FBSupervisorConfig{
		FluentBitExePath:     c.FluentBitExePath,
		FluentBitNRLibPath:   c.FluentBitNRLibPath,
		FluentBitParsersPath: c.FluentBitParsersPath,
		FluentBitVerbose:     c.Verbose != 0 && trace.IsEnabled(trace.LOG_FWD),
	}
	if fbIntCfg.IsLogForwarderAvailable() {
		logCfgLoader := logs.NewFolderLoader(logFwCfg, agt.Context.Identity, agt.Context.HostnameResolver())
		logSupervisor := v4.NewFBSupervisor(
			fbIntCfg,
			logCfgLoader,
			agt.Context.AgentIDUpdateNotifier(),
			agt.Context.HostnameChangeNotifier(),
			agt.Context.SendEvent,
		)
		go logSupervisor.Run(agt.Context.Ctx)
	} else {
		aslog.Debug("Log forwarder is not available for this platform. The agent will start without log forwarding support.")
	}

	ffHandle.SetOHIHandler(integrationManager)

	go integrationManager.Start(agt.Context.Ctx)

	go ccService.Run(agt.Context.Ctx, agt.Context.AgentIdnOrEmpty, initCmdResponse)

	//pluginConfig, err := legacy.LoadPluginConfig(pluginRegistry, c.PluginConfigFiles)
	//if err != nil {
	//	fatal(err, "Can't load plugin configuration.")
	//}
	//runner := legacy.NewPluginRunner(pluginRegistry, agt)
	//for _, pluginConf := range pluginConfig.PluginConfigs {
	//	if err := runner.ConfigurePlugin(pluginConf, agt.Context.ActiveEntitiesChannel()); err != nil {
	//		fatal(err, fmt.Sprint("Can't configure plugin.", pluginConf))
	//	}
	//}
	//
	//if err := runner.ConfigureV1Plugins(agt.Context); err != nil {
	//	aslog.WithError(err).Debug("Can't configure integrations.")
	//}

	timedLog.Info("New Relic infrastructure agent is running.")

	return agt.Run()
}

// initInstrumentation will spawn a server and expose agent metrics through prometheus exporter.
// By default is disabled and it only will be enabled if host:port are provided.
// Using instrumentation.SetupPrometheusIntegrationConfig it will create prometheus
// integration configuration (and delete it on agent shutdown process).
func initInstrumentation(ctx context2.Context, agentMetricsEndpoint string) (instrumentation.Instrumenter, error) {
	if agentMetricsEndpoint == "" {
		return instrumentation.NewNoop(), nil
	}

	instruments, err := instrumentation.New()
	if err != nil {
		return nil, err
	}

	aslog.WithField("addr", agentMetricsEndpoint).Info("Starting Opentelemetry server")
	srv := &http.Server{
		Handler:      instruments.GetHandler(),
		Addr:         agentMetricsEndpoint,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go srv.ListenAndServe()
	go func() {
		<-ctx.Done()
		aslog.Debug("Stopping Opentelemetry server")
		srv.Close()
	}()

	//Setup prometheus integration
	err = instrumentation.SetupPrometheusIntegrationConfig(ctx, agentMetricsEndpoint)
	if err != nil {
		return nil, err
	}

	return instruments, nil
}

// newInstancesLookup creates an instance lookup that:
// - looks in the v3 legacy definitions repository for defined commands
// - looks in the definition folders (and bin/ subfolders) for executable names
func newInstancesLookup(cfg v4.Configuration) integration.InstancesLookup {
	const executablesSubFolder = "bin"

	var execFolders []string
	for _, df := range cfg.DefinitionFolders {
		execFolders = append(execFolders, df)
		execFolders = append(execFolders, filepath.Join(df, executablesSubFolder))
	}
	legacyDefinedCommands := v3.NewDefinitionsRepo(v3.LegacyConfig{
		DefinitionFolders: cfg.DefinitionFolders,
		Verbose:           cfg.Verbose,
	})
	return integration.InstancesLookup{
		Legacy: legacyDefinedCommands.NewDefinitionCommand,
		ByName: files.Executables{Folders: execFolders}.Path,
	}
}

// configureLogFormat checks the config and sets the log format accordingly.
func configureLogFormat(logFormat string) {
	if logFormat == config.LogFormatJSON {
		jsonFormatter := &logrus.JSONFormatter{
			DataKey: "context",

			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime: "timestamp",
			},
		}
		wlog.SetFormatter(jsonFormatter)
	}
}

// Either route standard logging to stdout (for Linux, so it gets copied to syslog as appropriate)
// or copy it to stdout and a log file for Mac/Windows so we don't lose the logging when running
// as a service.
func configureLogRedirection(config *config.Config, memLog *wlog.MemLogger) (onFile bool) {
	if config.LogFile == "" && !(config.IsTroubleshootMode() && systemd.IsAgentRunningOnSystemD()) {
		wlog.SetOutput(os.Stdout)
	} else {
		// Redirect all output to both stdout and the agent's own log file.
		logFile, err := disk.OpenFile(config.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			alog.WithField("action", "configureLogRedirection").WithError(err).Error("Can't open log file.")
			os.Exit(1)
		}
		alog.WithFields(logrus.Fields{
			"action":      "configureLogRedirection",
			"logFile":     logFile.Name(),
			"logToStdout": config.LogToStdout,
		}).Debug("Redirecting output to a file.")
		// Write all previous logs, which are stored in memLog, to the file.
		_, err = memLog.WriteBuffer(logFile)
		if err != nil {
			wlog.WithError(err).Debug("Failed writing log to file.")
		} else {
			onFile = true
		}
		wlog.SetOutput(&fileAndConsoleLogger{logFile: logFile, stdout: config.LogToStdout})
	}
	return
}

// A simple logging wrapper which copies all output to both stdout and a log file to make it easier to find.
// This is nice for Windows, since there's nothing built-in to capture all stdout from a program into some
// kind of syslog, and we don't want to flood the system event log with uninteresting messages.
type fileAndConsoleLogger struct {
	logFile *os.File
	stdout  bool
}

func (fc *fileAndConsoleLogger) Write(b []byte) (n int, err error) {
	if fc.stdout {
		_, _ = os.Stdout.Write(b)
	}
	return fc.logFile.Write(b)
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
	transport http.RoundTripper) (timedOut bool, err error) {
	var request *http.Request
	if request, err = http.NewRequest("HEAD", collectorURL, nil); err != nil {
		return false, fmt.Errorf("unable to prepare reachability request: %v, error: %s", request, err)
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
