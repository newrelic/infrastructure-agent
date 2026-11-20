// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v4

import (
	ctx2 "context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
)

const fbVersionTimeout = 5 * time.Second

// fbMigrationDocURL points to the guide for migrating Fluent Bit classic configuration to YAML.
// TODO: placeholder, replace with the final documentation link.
const fbMigrationDocURL = "<FLUENT_BIT_YAML_MIGRATION_DOC_URL>"

// Fluent Bit versions that drive the config format selection.
const (
	// YAML config (including YAML parsers files) is fully supported since 3.2.
	fbYAMLMinMajor = 3
	fbYAMLMinMinor = 2
)

var (
	fbVersionRegex = regexp.MustCompile(`Fluent Bit v(\d+)\.(\d+)\.(\d+)`)

	errFbVersionNotFound          = errors.New("fluent bit version not found in output")
	errFbYAMLNotSupported         = errors.New("fluent bit version does not support YAML config")
	errFbConfFormatVsYAMLExternal = errors.New(
		"fluent_bit_config_format is conf but the external config file is YAML, " +
			"set fluent_bit_config_format to yaml or migrate the external file to classic format, see " +
			fbMigrationDocURL)
	errFbExternalYAMLNoVersion = errors.New("external fluent bit config is YAML but fluent bit version could not be detected")
)

// FBVersion is a Fluent Bit version as printed by "fluent-bit --version".
type FBVersion struct {
	Major, Minor, Patch int
}

func (v FBVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// AtLeast returns true if the version is greater or equal to major.minor.
func (v FBVersion) AtLeast(major, minor int) bool {
	return v.Major > major || (v.Major == major && v.Minor >= minor)
}

func (v FBVersion) supportsYAML() bool {
	return v.AtLeast(fbYAMLMinMajor, fbYAMLMinMinor)
}

// ParseFBVersion extracts the version from the "fluent-bit --version" output, ie: "Fluent Bit v5.1.2".
func ParseFBVersion(output string) (FBVersion, error) {
	matches := fbVersionRegex.FindStringSubmatch(output)
	if matches == nil {
		return FBVersion{}, errFbVersionNotFound
	}

	// regex guarantees digits
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return FBVersion{Major: major, Minor: minor, Patch: patch}, nil
}

// fbVersionDetector runs "fluent-bit --version" and caches the result per binary.
// The binary is checked again when it changes, as a package upgrade replaces it in place.
type fbVersionDetector struct {
	lock       sync.Mutex
	cache      map[string]cachedFBVersion
	runVersion func(ctx ctx2.Context, exePath string) ([]byte, error)
}

type cachedFBVersion struct {
	size    int64
	modTime time.Time
	version FBVersion
}

func newFBVersionDetector() *fbVersionDetector {
	return &fbVersionDetector{
		cache: map[string]cachedFBVersion{},
		runVersion: func(ctx ctx2.Context, exePath string) ([]byte, error) {
			return exec.CommandContext(ctx, exePath, "--version").CombinedOutput() //nolint:gosec
		},
	}
}

func (d *fbVersionDetector) Detect(exePath string) (FBVersion, error) {
	info, err := os.Stat(exePath)
	if err != nil {
		return FBVersion{}, fmt.Errorf("cannot stat fluent bit executable: %w", err)
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	if cached, ok := d.cache[exePath]; ok && cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		return cached.version, nil
	}

	ctx, cancel := ctx2.WithTimeout(ctx2.Background(), fbVersionTimeout)
	defer cancel()

	output, err := d.runVersion(ctx, exePath)
	if err != nil {
		return FBVersion{}, fmt.Errorf("cannot run fluent bit --version: %w", err)
	}

	version, err := ParseFBVersion(string(output))
	if err != nil {
		return FBVersion{}, err
	}

	d.cache[exePath] = cachedFBVersion{size: info.Size(), modTime: info.ModTime(), version: version}

	return version, nil
}

func describeFBVersion(version FBVersion, known bool) string {
	if !known {
		return "(unknown version)"
	}

	return "v" + version.String()
}

// selectFBConfigFormat picks the format following the customer's fluent_bit_config_format setting
// (userFormat: "yaml", "conf" or empty), the external config file and the Fluent Bit version.
// The yaml feature flag has the last word: while it is off the agent behaves as before, always classic.
// YAML and classic files can't include each other, so the external file's format has to be honored.
//
//nolint:cyclop
func selectFBConfigFormat(
	version FBVersion,
	versionErr error,
	externalCfg logs.FBCfgExternal,
	userFormat string,
	yamlFlag bool,
) (logs.FBConfigFormat, error) {
	if !yamlFlag {
		return logs.FBConfigFormatClassic, nil
	}

	versionKnown := versionErr == nil
	supportsYAML := versionKnown && version.supportsYAML()
	externalFormat := externalCfg.ConfigFormat()
	wantsYAML := userFormat == config.FluentBitConfigFormatYAML
	wantsConf := userFormat == config.FluentBitConfigFormatConf

	if versionKnown && !supportsYAML {
		if externalFormat == logs.FBConfigFormatYAML {
			return "", fmt.Errorf("%w: detected v%s, YAML requires v%d.%d or newer",
				errFbYAMLNotSupported, version, fbYAMLMinMajor, fbYAMLMinMinor)
		}
		if wantsYAML {
			sFBLogger.WithField("fluentBitVersion", "v"+version.String()).
				Warn("fluent_bit_config_format is yaml but this Fluent Bit version does not support it. Using classic.")
		}

		return logs.FBConfigFormatClassic, nil
	}

	switch externalFormat {
	case logs.FBConfigFormatYAML:
		if wantsConf {
			return "", fmt.Errorf("%w: %s", errFbConfFormatVsYAMLExternal, externalCfg.CfgFilePath)
		}
		if !versionKnown {
			// YAML picked only because the external file is YAML, without knowing if FB can read it
			sFBLogger.WithError(versionErr).Warn(errFbExternalYAMLNoVersion.Error())
		}

		return logs.FBConfigFormatYAML, nil
	case logs.FBConfigFormatClassic:
		// A classic file can't be included from YAML, so the external file's format wins.
		if wantsYAML {
			sFBLogger.WithField("configFile", externalCfg.CfgFilePath).
				Warn("fluent_bit_config_format is yaml but the external config file is classic. Using classic.")
		}
		sFBLogger.WithField("configFile", externalCfg.CfgFilePath).
			Warn("Using classic Fluent Bit configuration for the external config file. " +
				"Newer Fluent Bit features may require YAML; migrate your fluentbit config_file to YAML, see " +
				fbMigrationDocURL)

		return logs.FBConfigFormatClassic, nil
	}

	// no external config: an explicit setting wins over the feature flag
	switch {
	case wantsConf:
		return logs.FBConfigFormatClassic, nil
	case wantsYAML && supportsYAML:
		return logs.FBConfigFormatYAML, nil
	case wantsYAML:
		sFBLogger.WithError(versionErr).Warn("fluent_bit_config_format is yaml but the Fluent Bit version " +
			"could not be detected. Using classic.")

		return logs.FBConfigFormatClassic, nil
	case supportsYAML:
		return logs.FBConfigFormatYAML, nil
	default:
		return logs.FBConfigFormatClassic, nil
	}
}
