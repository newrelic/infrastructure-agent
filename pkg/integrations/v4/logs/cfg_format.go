// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FBConfigFormat is the Fluent Bit configuration file format the agent generates.
type FBConfigFormat string

const (
	// FBConfigFormatAuto lets the agent pick the format from the detected Fluent Bit version.
	FBConfigFormatAuto FBConfigFormat = "auto"
	// FBConfigFormatClassic is the legacy "[INPUT] / [FILTER] / [OUTPUT]" format (.conf).
	FBConfigFormatClassic FBConfigFormat = "classic"
	// FBConfigFormatYAML is the YAML format (.yaml), supported since Fluent Bit 3.2.
	FBConfigFormatYAML FBConfigFormat = "yaml"
)

// ParseFBConfigFormat parses the fluent_bit_config_format setting. Empty means auto.
func ParseFBConfigFormat(value string) (FBConfigFormat, error) {
	switch FBConfigFormat(strings.ToLower(strings.TrimSpace(value))) {
	case "", FBConfigFormatAuto:
		return FBConfigFormatAuto, nil
	case FBConfigFormatClassic:
		return FBConfigFormatClassic, nil
	case FBConfigFormatYAML:
		return FBConfigFormatYAML, nil
	default:
		return FBConfigFormatAuto, fmt.Errorf("invalid fluent bit config format %q, expected auto, classic or yaml", value) //nolint:err113
	}
}

// FileExtension returns the extension Fluent Bit uses to pick the config parser.
// Fluent Bit treats any file not ending in .yaml/.yml as classic, so it must be set.
func (f FBConfigFormat) FileExtension() string {
	if f == FBConfigFormatYAML {
		return ".yaml"
	}

	return ".conf"
}

// ConfigFormat returns the format of the external Fluent Bit config file, based on its extension.
// It returns an empty format when there is no external config file.
func (c FBCfgExternal) ConfigFormat() FBConfigFormat {
	if c.CfgFilePath == "" {
		return ""
	}

	switch strings.ToLower(filepath.Ext(c.CfgFilePath)) {
	case ".yaml", ".yml":
		return FBConfigFormatYAML
	default:
		return FBConfigFormatClassic
	}
}

// FormatAs returns the FBCfg rendered in the requested Fluent Bit config format.
func (c FBCfg) FormatAs(format FBConfigFormat) (string, FBCfgExternal, error) {
	if format == FBConfigFormatYAML {
		return c.FormatYAML()
	}

	return c.Format()
}
