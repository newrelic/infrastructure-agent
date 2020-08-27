// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package handler

import (
	"os"

	"github.com/newrelic/infrastructure-agent/internal/os/api"
	"github.com/newrelic/infrastructure-agent/pkg/trace"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"

	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	// FFs
	FlagCategory             = "Infra_Agent"
	FlagNameRegister         = "register_enabled"
	FlagParallelizeInventory = "parallelize_inventory_enabled"
	FlagProtocolV4           = "protocol_v4_enabled"
	FlagFullProcess          = "full_process_sampling"
	// Config
	CfgYmlRegisterEnabled        = "register_enabled"
	CfgYmlParallelizeInventory   = "inventory_queue_len"
	CfgValueParallelizeInventory = int64(100) // default value when no config provided by user and FF enabled
)

var ffLogger = log.WithComponent("FeatureFlagHandler")

// FFHandler handles FF commands.
type FFHandler struct {
	cfg        *config.Config
	ohiEnabler OHIEnabler
	ffSetter   feature_flags.Setter
	ffsState   handledStatePerFF
}

// OHIEnabler enables or disables an OHI via cmd-channel feature flag.
type OHIEnabler interface {
	EnableOHIFromFF(featureFlag string) error
	DisableOHIFromFF(featureFlag string) error
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

// NewFFHandler creates a new feature-flag cmd handler, FFHandler not available at this time.
func NewFFHandler(cfg *config.Config, ffSetter feature_flags.Setter) *FFHandler {
	return &FFHandler{
		cfg:      cfg,
		ffsState: make(handledStatePerFF),
		ffSetter: ffSetter,
	}
}

// SetOHIHandler injects the handler dependency. A proper refactor of agent services injection will
// be required for this to be injected via srv constructor.
func (h *FFHandler) SetOHIHandler(e OHIEnabler) {
	h.ohiEnabler = e
}

func (h *FFHandler) Handle(ffArgs commandapi.FFArgs, isInitialFetch bool) {
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

	// this is where we handle normal feature flags that are not related to OHIs
	if ffArgs.Flag == FlagProtocolV4 || ffArgs.Flag == FlagFullProcess {
		h.setFFConfig(ffArgs.Flag, ffArgs.Enabled)
		return
	}

	// integration enabler won't be ready at initial fetch
	if isInitialFetch {
		return
	}

	// evaluated at the end as integration name flag is looked up dynamically
	h.handleEnableOHI(ffArgs.Flag, ffArgs.Enabled)
}

func (h *FFHandler) setFFConfig(ff string, enabled bool) {
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

func (h *FFHandler) handleEnableOHI(ff string, enable bool) {
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
		err = h.ohiEnabler.EnableOHIFromFF(ff)
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

func handleParallelizeInventory(ffArgs commandapi.FFArgs, c *config.Config, isInitialFetch bool) {
	trace.Inventory("parallelize FF handler initialFetch: %v, enable: %v, queue: %v",
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

func handleRegister(ffArgs commandapi.FFArgs, c *config.Config, isInitialFetch bool) {
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
