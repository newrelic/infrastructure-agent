// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"compress/gzip"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/newrelic/infrastructure-agent/cmd/newrelic-infra/initialize"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/instrumentation"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/files"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/v3legacy"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/track"
	v4 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4"
	v4Config "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/sirupsen/logrus"
)

type Emulator struct {
	chRequests     chan http.Request
	agent          *agent.Agent
	integrationCfg v4.ManagerConfig
	tempDir        string
}

func (ae *Emulator) ChannelHTTPRequests() chan http.Request {
	return ae.chRequests
}

func New(configsDir, tempBinDir string) *Emulator {
	rc := ihttp.NewRequestRecorderClient()

	logToStdout := true
	ag := infra.NewAgent(rc.Client, func(conf *config.Config) {
		conf.DisplayName = "my_display_name"
		conf.License = "abcdef012345"
		conf.PayloadCompressionLevel = gzip.NoCompression
		conf.Log.Level = config.LogLevelDebug
		conf.PluginDir = configsDir
		conf.Log.Format = "text"
		conf.Log.ToStdout = &logToStdout
		conf.IsForwardOnly = true
		conf.Features = map[string]bool{
			fflag.FlagProtocolV4: true,
		}
		conf.CustomPluginInstallationDir = tempBinDir
	})
	cfg := ag.Context.Config()
	integrationCfg := v4.NewManagerConfig(
		cfg.Log.VerboseEnabled(),
		cfg.Features,
		cfg.PassthroughEnvironment,
		[]string{configsDir},
		[]string{cfg.CustomPluginInstallationDir},
	)

	return &Emulator{
		chRequests:     rc.RequestCh,
		agent:          ag,
		integrationCfg: integrationCfg,
		tempDir:        tempBinDir,
	}
}

func (ae *Emulator) Terminate() {
	ae.agent.Context.CancelFn()
	ae.agent.Terminate()
	os.RemoveAll(ae.tempDir)
}

func (ae *Emulator) RunAgent() error {
	malog := logrus.WithField("component", "minimal-standalone-agent")
	logrus.Info("Runing minimalistic test agent...")
	runtime.GOMAXPROCS(1)

	cfg := ae.agent.GetContext().Config()

	ffManager := feature_flags.NewManager(cfg.Features)
	fatal := func(err error, message string) {
		malog.WithError(err).Error(message)
		os.Exit(1)
	}

	if err := initialize.AgentService(cfg); err != nil {
		fatal(err, "Can't complete platform specific initialization.")
	}
	metricsSenderConfig := dm.NewConfig(cfg.MetricURL, false, cfg.License, time.Duration(cfg.DMSubmissionPeriod)*time.Second, cfg.MaxMetricBatchEntitiesCount, cfg.MaxMetricBatchEntitiesQueue)
	dmSender, err := dm.NewDMSender(metricsSenderConfig, http.DefaultTransport, ae.agent.Context.IdContext().AgentIdentity)
	if err != nil {
		return err
	}

	// queues integration run requests
	definitionQ := make(chan integration.Definition, 100)
	// queues config entries requests
	configEntryQ := make(chan configrequest.Entry, 100)

	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	dmEmitter := dm.NewEmitter(ae.agent.GetContext(), dmSender, nil, instrumentation.NoopMeasure, ffRetriever)

	// track stoppable integrations
	tracker := track.NewTracker(dmEmitter)
	il := newInstancesLookup(ae.integrationCfg)
	integrationEmitter := emitter.NewIntegrationEmittor(ae.agent, dmEmitter, ffManager)
	integrationManager := v4.NewManager(
		ae.integrationCfg,
		v4Config.NewPathLoader(),
		integrationEmitter,
		il,
		definitionQ,
		configEntryQ,
		tracker,
		ae.agent.Context.IDLookup(),
	)

	// Start all plugins we want the agent to run.
	if err = plugins.RegisterPlugins(ae.agent); err != nil {
		malog.WithError(err).Error("fatal error while registering plugins")
		os.Exit(1)
	}
	go integrationManager.Start(ae.agent.Context.Ctx)
	go func() {
		if err := ae.agent.Run(); err != nil {
			panic(err)
		}
	}()

	return nil
}

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
	})
	return integration.InstancesLookup{
		Legacy: legacyDefinedCommands.NewDefinitionCommand,
		ByName: files.Executables{Folders: execFolders}.Path,
	}
}
