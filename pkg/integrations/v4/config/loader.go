package config

import (
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/files"
	"github.com/newrelic/infrastructure-agent/pkg/config/envvar"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

var clLog = log.WithComponent("integrations.config.Loader")

// LegacyYAML is not an actual error. Used for discarding V3 plugins
var LegacyYAML = errors.New("file format belongs to the old integrations format")

const (
	// Integrations V3 configs use the "instances" key word
	// In the current Integrations config, we use "integrations"
	// The two fields below will allows us distinguish between both, to ignore V4
	integrationsField    = "integrations"
	LegacyInstancesField = "instances"
)

// Loader will read and parse integrations v4 config files.
type Loader interface {
	// Load reads all the configuration files in a given directory. If path is a file instead of a directory
	// will try to read it as a single configuration file.
	Load(path string) (YAMLMap, error)

	// LoadFile reads the configuration file and discards it if it's not belonging to the V4 format
	LoadFile(file string) (ok bool, cfg YAML)
}

type pathLoader struct {
	isDirectory func(path string) (bool, error)
}

// NewPathLoader returns a new instance of a config Loader.
func NewPathLoader() Loader {
	return &pathLoader{
		isDirectory: isDirectory,
	}
}

// Load reads all the configuration files in a given directory. If path is a file instead of a directory
// will try to read it as a single configuration file.
func (pl *pathLoader) Load(path string) (YAMLMap, error) {
	isDir, err := isDirectory(path)
	if err != nil {
		return nil, err
	}
	if isDir {
		return pl.loadDirectory(path)
	}

	ok, cfg := pl.LoadFile(path)
	if !ok {
		return nil, fmt.Errorf("no configuration found in path %s", path)
	}

	return YAMLMap{
		path: cfg,
	}, nil
}

// LoadFile reads the configuration file and discards it if it's not belonging to the V4 format
func (pl *pathLoader) LoadFile(file string) (bool, YAML) {
	fLog := clLog.WithField("file", file)

	cfg, err := loadFile(file)
	if err != nil {
		if err == LegacyYAML {
			fLog.Debug("Skipping v3 integration.")
		} else {
			fLog.WithError(err).Warn("can't load integrations file. This may happen if you are editing a file and saving intermediate changes")
		}
	}
	return err == nil, cfg
}

// loadDirectory reads the configuration files in a given directory, and discards those not belonging to the V4 format
func (pl *pathLoader) loadDirectory(dir string) (YAMLMap, error) {
	dLog := clLog.WithField("dir", dir)

	yamlFiles, err := files.AllYAMLs(dir)
	if err != nil {
		return nil, err
	}

	configs := YAMLMap{}
	for _, file := range yamlFiles {
		fLog := dLog.WithField("file", file.Name())
		absolutePath := filepath.Join(dir, file.Name())

		fLog.Debug("Loading config.")
		if ok, cfg := pl.LoadFile(absolutePath); ok {
			configs[absolutePath] = cfg
		}
	}
	return configs, nil
}

// loadFile will read the file located in path and will try to parse it as yaml.
func loadFile(path string) (YAML, error) {
	cy := YAML{}
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return cy, err
	}

	bytes, err = envvar.ExpandInContent(bytes)
	if err != nil {
		return cy, err
	}

	if err := yaml.Unmarshal(bytes, &cy); err != nil {
		return cy, err
	}
	if len(cy.Integrations) == 0 {
		return cy, explainEmptyIntegrations(bytes)
	}
	return cy, nil
}

// returns why a v4 integration is empty: because it's a v3 integration or because it has a wrong format
func explainEmptyIntegrations(bytes []byte) error {
	var contents map[string]interface{}
	err := yaml.Unmarshal(bytes, &contents)
	if err != nil {
		return err // should never happen
	}
	if _, ok := contents[LegacyInstancesField]; ok {
		return LegacyYAML
	}
	if _, ok := contents[integrationsField]; ok {
		return errors.New("'" + integrationsField + "' seems to be empty or wrongly formatted")
	}
	return errors.New("missing '" + integrationsField + "' field")
}

// isDirectory determines if a file represented
// by `path` is a directory or not.
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
