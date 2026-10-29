// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package health reports the health of the integrations declared in a single integration
// configuration file into a file named after that configuration file. The files are meant to be
// consumed by a supervisor such as Agent Control, which can then tell which integration is
// failing instead of only whether the agent itself is up.
package health

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	dirPerm = 0o755
	// the health files are read by another process, so they must not be restricted to the writing user
	filePerm = 0o644

	statusNotRunYet  = "No integration execution recorded yet"
	statusAllHealthy = "All integrations healthy"
	noDataLastError  = "integration ran without producing metrics, events or inventory"
)

// ErrEmptyDir is returned when a Writer is requested without an output directory.
var ErrEmptyDir = errors.New("health directory cannot be empty")

// Report is the health document written to disk. The YAML keys are an external contract: they are
// the ones Agent Control's file health checker reads.
type Report struct {
	Healthy            bool   `yaml:"healthy"`
	Status             string `yaml:"status"`
	LastError          string `yaml:"last_error,omitempty"`
	StartTimeUnixNano  int64  `yaml:"start_time_unix_nano"`
	StatusTimeUnixNano int64  `yaml:"status_time_unix_nano"`
}

// Writer writes health reports into a directory, one file per name.
type Writer struct {
	dir string
}

// NewWriter creates the output directory if it does not already exist.
func NewWriter(dir string) (*Writer, error) {
	if dir == "" {
		return nil, ErrEmptyDir
	}

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating health directory %q: %w", dir, err)
	}

	return &Writer{dir: dir}, nil
}

// Dir returns the directory the health files are written into.
func (w *Writer) Dir() string {
	return w.dir
}

// Write replaces the file named name atomically, so that a reader never observes a partially
// written document.
func (w *Writer) Write(name string, r Report) error {
	contents, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshaling health report: %w", err)
	}

	tmp, err := os.CreateTemp(w.dir, name+".tmp")
	if err != nil {
		return fmt.Errorf("creating temporary health file in %q: %w", w.dir, err)
	}

	if err := writeAndClose(tmp, contents); err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("writing temporary health file %q: %w", tmp.Name(), err)
	}

	if err := os.Rename(tmp.Name(), filepath.Join(w.dir, name)); err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("renaming temporary health file %q into place: %w", tmp.Name(), err)
	}

	return nil
}

// Remove deletes the health file named name. A file that is already gone is not an error.
func (w *Writer) Remove(name string) error {
	if err := os.Remove(filepath.Join(w.dir, name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing health file %q: %w", name, err)
	}

	return nil
}

func writeAndClose(f *os.File, contents []byte) error {
	defer f.Close()

	if err := f.Chmod(filePerm); err != nil {
		return err
	}

	if _, err := f.Write(contents); err != nil {
		return err
	}

	return f.Sync()
}

// Result is what one execution round of one integration produced.
type Result struct {
	// Executions is the number of integration processes the round started. Zero means the round did
	// not run anything: no discovery match, or unmet "when" conditions.
	Executions int
	// DataPayloads is the number of emitted payloads that carried at least one metric, event or
	// inventory item.
	DataPayloads int
	// Errors holds the already obfuscated error messages the round produced.
	Errors []string
}

type namedResult struct {
	name   string
	result Result
}

// Reporter collapses the latest result of every integration declared in one integration
// configuration file into a single health file named after that configuration file.
type Reporter struct {
	fileName string
	writer   *Writer
	start    time.Time
	now      func() time.Time

	mu     sync.Mutex
	latest map[string]namedResult
}

// NewReporter returns a Reporter writing to fileName inside the writer's directory. now is
// injectable for testing and defaults to time.Now.
func NewReporter(fileName string, w *Writer, start time.Time, now func() time.Time) *Reporter {
	if now == nil {
		now = time.Now
	}

	return &Reporter{
		fileName: fileName,
		writer:   w,
		start:    start,
		now:      now,
		latest:   make(map[string]namedResult),
	}
}

// Init writes the health file before any integration has run, so that the file exists from the
// moment the configuration file is loaded. Agent Control reads a missing health file as a failing
// check, which would otherwise be the permanent state of a configuration file declaring no
// integration at all.
func (r *Reporter) Init() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.writeLocked()
}

// Record stores the latest result of the integration identified by key and rewrites the health
// file. name is the integration name, reported in the status.
func (r *Reporter) Record(key, name string, res Result) error {
	// The lock is held across the write: the integrations of one configuration file are run by
	// concurrent runners sharing this reporter, and a report computed before a failure was recorded
	// must never be the one that ends up on disk.
	r.mu.Lock()
	defer r.mu.Unlock()

	r.latest[key] = namedResult{name: name, result: res}

	return r.writeLocked()
}

// writeLocked writes the current verdict. Callers must hold r.mu.
func (r *Reporter) writeLocked() error {
	return r.writer.Write(r.fileName, r.aggregateLocked())
}

// Remove deletes the health file, for when its integration configuration file is gone.
func (r *Reporter) Remove() error {
	return r.writer.Remove(r.fileName)
}

// aggregateLocked collapses every integration's latest result into one report. The first unhealthy
// integration wins, mirroring how Agent Control collapses its own list of health checks. Callers
// must hold r.mu.
func (r *Reporter) aggregateLocked() Report {
	report := Report{
		Healthy:            true,
		Status:             statusNotRunYet,
		StartTimeUnixNano:  r.start.UnixNano(),
		StatusTimeUnixNano: r.now().UnixNano(),
	}

	if len(r.latest) == 0 {
		return report
	}

	report.Status = statusAllHealthy

	for _, key := range r.sortedKeys() {
		switch current := r.latest[key]; {
		case len(current.result.Errors) > 0:
			report.Healthy = false
			report.Status = current.name + ": execution failed"
			report.LastError = current.result.Errors[0]

			return report
		// An integration that ran and emitted nothing is unhealthy. A round that started no process
		// at all is not: on a host where the monitored service is legitimately absent, the
		// integration is not broken.
		case current.result.Executions > 0 && current.result.DataPayloads == 0:
			report.Healthy = false
			report.Status = current.name + ": no data"
			report.LastError = noDataLastError

			return report
		}
	}

	return report
}

// sortedKeys orders the integrations by name and then by key, so that when several of them are
// unhealthy at once the reported one is always the same and the status does not flap between
// writes. Callers must hold r.mu.
func (r *Reporter) sortedKeys() []string {
	keys := make([]string, 0, len(r.latest))
	for key := range r.latest {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		if r.latest[keys[i]].name != r.latest[keys[j]].name {
			return r.latest[keys[i]].name < r.latest[keys[j]].name
		}

		return keys[i] < keys[j]
	})

	return keys
}

// dataSet is the part of one integration data set that says whether it carries anything. The keys
// are the same in every protocol version.
type dataSet struct {
	Metrics   []json.RawMessage          `json:"metrics"`
	Events    []json.RawMessage          `json:"events"`
	Inventory map[string]json.RawMessage `json:"inventory"`
}

func (d dataSet) hasData() bool {
	return len(d.Metrics) > 0 || len(d.Events) > 0 || len(d.Inventory) > 0
}

// payloadProbe matches both payload shapes at once: protocol v1 holds a single data set at the top
// level, protocols v2, v3 and v4 hold an array of data sets under "data".
type payloadProbe struct {
	Metrics   []json.RawMessage          `json:"metrics"`
	Events    []json.RawMessage          `json:"events"`
	Inventory map[string]json.RawMessage `json:"inventory"`
	Data      []dataSet                  `json:"data"`
}

// HasData reports whether an integration payload carried at least one metric, event or inventory
// item. A payload that cannot be parsed counts as no data: the emitter reports the parse failure
// through its own error path, and that error is what ends up in the health report.
func HasData(payload []byte) bool {
	var probe payloadProbe
	if err := json.Unmarshal(payload, &probe); err != nil {
		return false
	}

	topLevel := dataSet{Metrics: probe.Metrics, Events: probe.Events, Inventory: probe.Inventory}
	if topLevel.hasData() {
		return true
	}

	for _, ds := range probe.Data {
		if ds.hasData() {
			return true
		}
	}

	return false
}
