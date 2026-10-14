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

	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/logs"
)

const fbVersionTimeout = 5 * time.Second

// Fluent Bit versions that drive the config format selection.
const (
	// YAML config (including YAML parsers files) is fully supported since 3.2.
	fbYAMLMinMajor = 3
	fbYAMLMinMinor = 2
	// First Fluent Bit version without classic config support. Classic is deprecated at the end
	// of 2026 and the removal release is not announced yet: 6.0.0 is an assumption to revisit.
	fbNoClassicMajor = 6
	fbNoClassicMinor = 0
)

var (
	fbVersionRegex = regexp.MustCompile(`Fluent Bit v(\d+)\.(\d+)\.(\d+)`)

	errFbVersionNotFound       = errors.New("fluent bit version not found in output")
	errFbYAMLNotSupported      = errors.New("fluent bit version does not support YAML config")
	errFbClassicNotSupported   = errors.New("fluent bit version does not support classic config")
	errFbExternalCfgMismatch   = errors.New("external fluent bit config format does not match the selected format")
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

func (v FBVersion) supportsClassic() bool {
	return !v.AtLeast(fbNoClassicMajor, fbNoClassicMinor)
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

// selectFBConfigFormat decides which config format to generate.
//
// With "auto":
//   - customer external config in YAML: YAML, as classic cannot include YAML files.
//   - version unknown: classic, which is the behaviour before version detection existed.
//   - version below 6.0.0: classic.
//   - version 6.0.0 or newer (no classic support): YAML.
//
// A forced format or an external config file that the selected format cannot include returns an error.
//
//nolint:cyclop
func selectFBConfigFormat(
	setting logs.FBConfigFormat,
	version FBVersion,
	versionErr error,
	externalCfg logs.FBCfgExternal,
) (logs.FBConfigFormat, error) {
	versionKnown := versionErr == nil
	externalFormat := externalCfg.ConfigFormat()

	format := setting
	if format == logs.FBConfigFormatAuto || format == "" {
		switch {
		case externalFormat == logs.FBConfigFormatYAML:
			format = logs.FBConfigFormatYAML
		case !versionKnown || version.supportsClassic():
			format = logs.FBConfigFormatClassic
		default:
			format = logs.FBConfigFormatYAML
		}
	}

	if format == logs.FBConfigFormatYAML && versionKnown && !version.supportsYAML() {
		return "", fmt.Errorf("%w: detected v%s, YAML requires v%d.%d or newer",
			errFbYAMLNotSupported, version, fbYAMLMinMajor, fbYAMLMinMinor)
	}

	if format == logs.FBConfigFormatYAML && !versionKnown && setting != logs.FBConfigFormatYAML {
		// auto picked YAML only because the external file is YAML, without knowing if FB can read it
		sFBLogger.WithError(versionErr).Warn(errFbExternalYAMLNoVersion.Error())
	}

	if format == logs.FBConfigFormatClassic && versionKnown && !version.supportsClassic() {
		return "", fmt.Errorf("%w: detected v%s", errFbClassicNotSupported, version)
	}

	if externalFormat != "" && externalFormat != format {
		return "", fmt.Errorf("%w: %s is %s, but the agent generates %s config for fluent bit %s, "+
			"please provide a %s file", errFbExternalCfgMismatch, externalCfg.CfgFilePath, externalFormat,
			format, describeFBVersion(version, versionKnown), format)
	}

	return format, nil
}
