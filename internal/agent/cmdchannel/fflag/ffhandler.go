// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fflag

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/os/api"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	// FFs
	FlagCategory              = "Infra_Agent"
	FlagNameRegister          = "register_enabled"
	FlagParallelizeInventory  = "parallelize_inventory_enabled"
	FlagAsyncInventoryHandler = "async_inventory_handler_enabled"

	FlagProtocolV4           = "protocol_v4_enabled"
	FlagFullProcess          = "full_process_sampling"
	FlagDmRegisterDeprecated = "dm_register_deprecated"
	FlagFluentBit19          = "fluent_bit_19_win"
	// Config
	CfgYmlRegisterEnabled              = "register_enabled"
	CfgYmlParallelizeInventory         = "inventory_queue_len"
	CfgYmlAsyncInventoryHandlerEnabled = "async_inventory_handler_enabled"
	CfgValueParallelizeInventory       = int64(100) // default value when no config provided by user and FF enabled
)

//nolint:gochecknoglobals
var ffLogger = log.WithComponent("FeatureFlagHandler")

type args struct {
	Category string
	Flag     string
	Enabled  bool
}

// handler handles FF commands.
type handler struct {
	cfg         *config.Config
	ohiEnabler  OHIEnabler
	fbRestarter FBRestarter
	ffSetter    feature_flags.Setter
	ffsState    handledStatePerFF
	logger      log.Entry
}

// OHIEnabler enables or disables an OHI via cmd-channel feature flag.
type OHIEnabler interface {
	EnableOHIFromFF(ctx context.Context, featureFlag string) error
	DisableOHIFromFF(featureFlag string) error
}

type FBRestarter interface {
	Restart() error
}

// represents the state of an FF: non handled, handled to enable, handled to disable or both.
type ffHandledState uint

// returns if provided request has already been logged (to enable or disable), and updates state.
func (s *ffHandledState) requestWasAlreadyLogged(enabled bool) bool {
	if *s == ffHandledEnableAndDisableState {
		return true
	}

	if *s == ffNotHandledState {
		if enabled {
			*s = ffHandledEnabledState
		} else {
			*s = ffHandledDisabledState
		}
		return false
	}

	if *s == ffHandledEnabledState && enabled {
		return true
	}

	if *s == ffHandledDisabledState && !enabled {
		return true
	}

	if *s == ffHandledEnabledState && !enabled || *s == ffHandledDisabledState && enabled {
		*s = ffHandledEnableAndDisableState
	}

	return false
}

const (
	ffNotHandledState ffHandledState = iota
	ffHandledEnabledState
	ffHandledDisabledState
	ffHandledEnableAndDisableState
)

// indexed per FF name: a FF cmd has already been handled if there's an entry, value stores state.
type handledStatePerFF map[string]ffHandledState

// NewHandler creates a new feature-flag cmd handler, handler not available at this time.
func NewHandler(cfg *config.Config, ffSetter feature_flags.Setter, logger log.Entry) *handler {
	return &handler{
		cfg:      cfg,
		ffsState: make(handledStatePerFF),
		ffSetter: ffSetter,
		logger:   logger,
	}
}

// SetOHIHandler injects the handler dependency. A proper refactor of agent services injection will
// be required for this to be injected via srv constructor.
func (h *handler) SetOHIHandler(e OHIEnabler) {
	h.ohiEnabler = e
}

// SetFBRestarter injects the handler dependency. A proper refactor of agent services injection will
// be required for this to be injected via srv constructor.
func (h *handler) SetFBRestarter(fbr FBRestarter) {
	h.fbRestarter = fbr
}

func (h *handler) Handle(ctx context.Context, c commandapi.Command, isInitialFetch bool) (err error) {
	var ffArgs args
	if err = json.Unmarshal(c.Args, &ffArgs); err != nil {
		err = cmdchannel.NewArgsErr(err)
		return
	}

	if ffArgs.Category != FlagCategory {
		return
	}

	if ffArgs.Flag == FlagParallelizeInventory {
		handleParallelizeInventory(ffArgs, h.cfg, isInitialFetch)
		return
	}

	if ffArgs.Flag == FlagNameRegister {
		handleRegister(ffArgs, h.cfg, isInitialFetch)

		return
	}

	if ffArgs.Flag == FlagFluentBit19 {
		h.handleFBRestart(ffArgs)

		return
	}

	if ffArgs.Flag == FlagAsyncInventoryHandler {
		handleAsyncInventoryHandlerEnabled(ffArgs, h.cfg, isInitialFetch)
		return
	}

	// this is where we handle normal feature flags that are not related to OHIs. These are meant to just enable/disable
	// the falue of the feature flag
	if isBasicFeatureFlag(ffArgs.Flag) {
		h.setFFConfig(ffArgs.Flag, ffArgs.Enabled)
		return
	}

	// integration enabler won't be ready at initial fetch
	if isInitialFetch {
		return
	}

	// evaluated at the end as integration name flag is looked up dynamically
	h.handleEnableOHI(ctx, ffArgs.Flag, ffArgs.Enabled)

	return
}

// isBasicFeatureFlag will return if the FF has no other logic than being enabled/disabled.
func isBasicFeatureFlag(flag string) bool {
	return flag == FlagProtocolV4 ||
		flag == FlagFullProcess ||
		flag == FlagDmRegisterDeprecated
}

func (h *handler) setFFConfig(ff string, enabled bool) {
	err := h.ffSetter.SetFeatureFlag(ff, enabled)
	if err != nil {
		// ignore if the FF has been already set
		if err != feature_flags.ErrFeatureFlagAlreadyExists {
			ffLogger.
				WithError(err).
				WithField("feature_flag", ff).
				WithField("enable", enabled).
				Debug("Cannot set feature flag configuration.")
		}
	}
}

func (h *handler) handleEnableOHI(ctx context.Context, ff string, enable bool) {
	// customer agent config takes precedence
	if _, ok := h.cfg.Features[ff]; ok {
		return
	}

	if h.ohiEnabler == nil {
		ffLogger.
			WithField("feature_flag", ff).
			WithField("enable", enable).
			Debug("No OHI handler for cmd feature request.")
		return
	}

	var err error
	if enable {
		err = h.ohiEnabler.EnableOHIFromFF(ctx, ff)
	} else {
		err = h.ohiEnabler.DisableOHIFromFF(ff)
	}
	if err != nil {
		if ffState, ok := h.ffsState[ff]; !ok || !ffState.requestWasAlreadyLogged(enable) {
			ffLogger.
				WithError(err).
				WithField("feature_flag", ff).
				WithField("enable", enable).
				Debug("Unable to enable/disable OHI feature.")
		}
	}
}

func handleParallelizeInventory(ffArgs args, c *config.Config, isInitialFetch bool) {
	ffLogger.
		WithField(config.TracesFieldName, config.FeatureTrace).
		Tracef("parallelize FF handler initialFetch: %v, enable: %v, inventory queue: %v",
			isInitialFetch,
			ffArgs.Enabled,
			c.InventoryQueueLen,
		)
	// feature already in desired state
	if (ffArgs.Enabled && c.InventoryQueueLen > 0) || (!ffArgs.Enabled && c.InventoryQueueLen == 0) {
		return
	}

	if !isInitialFetch {
		os.Exit(api.ExitCodeRestart)
	}

	v := int64(0)
	if ffArgs.Enabled {
		v = CfgValueParallelizeInventory
	}

	if err := c.SetIntValueByYamlAttribute(CfgYmlParallelizeInventory, v); err != nil {
		ffLogger.
			WithError(err).
			WithField("field", CfgYmlParallelizeInventory).
			Warn("unable to update config value")
	}
}

func (h *handler) handleFBRestart(ffArgs args) {
	err := h.ffSetter.SetFeatureFlag(ffArgs.Flag, ffArgs.Enabled)
	if err != nil {
		// ignore if the FF has been already set
		if errors.Is(err, feature_flags.ErrFeatureFlagAlreadyExists) {
			return
		}

		ffLogger.
			WithError(err).
			WithField("feature_flag", ffArgs.Flag).
			WithField("enable", ffArgs.Enabled).
			Debug("Cannot set feature flag configuration.")

		return
	}

	if h.fbRestarter == nil {
		ffLogger.Debug("No fbRestarter for cmd feature request.")

		return
	}

	err = h.fbRestarter.Restart()
	if err != nil {
		ffLogger.
			WithError(err).
			WithField("enabled", ffArgs.Enabled).
			Debug("Unable to restart fb")
	}
}

func handleRegister(ffArgs args, c *config.Config, isInitialFetch bool) {
	if ffArgs.Enabled == c.RegisterEnabled {
		return
	}

	if !isInitialFetch {
		os.Exit(api.ExitCodeRestart)
	}

	if err := c.SetBoolValueByYamlAttribute(CfgYmlRegisterEnabled, ffArgs.Enabled); err != nil {
		ffLogger.
			WithError(err).
			WithField("field", CfgYmlRegisterEnabled).
			Warn("unable to update config value")
	}
}

func handleAsyncInventoryHandlerEnabled(ffArgs args, c *config.Config, isInitialFetch bool) {
	// feature already in desired state.
	if ffArgs.Enabled == c.AsyncInventoryHandlerEnabled {
		return
	}

	if !isInitialFetch {
		os.Exit(api.ExitCodeRestart)
	}

	if err := c.SetBoolValueByYamlAttribute(CfgYmlAsyncInventoryHandlerEnabled, ffArgs.Enabled); err != nil {
		ffLogger.
			WithError(err).
			WithField("field", CfgYmlAsyncInventoryHandlerEnabled).
			Warn("unable to update config value")
	}
}
