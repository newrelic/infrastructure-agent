// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package testhelpers provide some helper functions that are useful for testing
package testhelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// FakeDeltaEntry provides meta information about a plugin deltas set
type FakeDeltaEntry struct {
	// Source is the categorized name of the plugin, e.g. 'packages/rpm'
	Source string
	// DeltasSize is the approximate size of all the unsent deltas for this plugin
	DeltasSize int
	// BodySize is the approximate size of the inventory JSON body for this plugin
	BodySize int
}

// PopulateDeltas creates a set of synthetic, fake deltas for a given entity, in the given data directory.
// The deltas will be inserted in the dataDir/.delta_repo folder.
func PopulateDeltas(dataDir string, entityKey string, deltasInfos []FakeDeltaEntry) {
	for _, plugin := range deltasInfos {
		categoryTerm := strings.Split(plugin.Source, "/")
		body := map[string]interface{}{}
		size := 0
		deltasNum := 0
		// Create the JSON body of the fake plugin
		var bodyJson []byte
		var err error
		for size < plugin.BodySize {
			key := fmt.Sprintf("key%v", deltasNum)
			value := fmt.Sprintf("value%v", deltasNum)
			body[key] = value
			bodyJson, err = json.Marshal(body)
			panicIfErr(err)
			size = len(bodyJson)
			deltasNum++
		}
		// Write it into the data directory
		deltaDir := path.Join(dataDir, ".delta_repo", categoryTerm[0], helpers.SanitizeFileName(entityKey))
		panicIfErr(os.MkdirAll(deltaDir, 0755))
		file, err := os.OpenFile(path.Join(deltaDir, categoryTerm[1]+".json"), os.O_CREATE|os.O_WRONLY, 0644)
		panicIfErr(err)
		_, err = file.Write(bodyJson)
		_ = file.Close()
		panicIfErr(err)

		// Then write a lot of deltas for this directory
		file, err = os.OpenFile(path.Join(deltaDir, categoryTerm[1]+".pending"), os.O_CREATE|os.O_WRONLY, 0644)
		panicIfErr(err)
		size = 0
		delta := inventoryapi.RawDelta{
			Source:    plugin.Source,
			ID:        0,
			Timestamp: time.Now().Unix(),
			Diff:      body,
			FullDiff:  false,
		}
		for size < plugin.DeltasSize {
			deltaJson, err := json.Marshal(delta)
			panicIfErr(err)
			deltaJson = append(deltaJson, ',')
			size += len(deltaJson)
			_, err = file.Write(deltaJson)
			delta.ID++
			delta.Timestamp++
		}
		file.Close()
	}
}

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

// PostDeltaTracer traces metadata about invocations to a fake inventory ingest service
type PostDeltaTracer struct {
	Sources          []map[string]interface{}
	Errors           []error
	PostDeltas       func(_ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error)
	PostDeltasVortex func(_ entity.ID, _ []string, _ bool, _ ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error)
}

// NewPostDeltaTracer creates a PostDeltaTracer whose PostDeltas method returns an error if the total submitted deltas are
// larger than the maxDeltaSize parameter.
func NewPostDeltaTracer(maxDeltaSize int) *PostDeltaTracer {
	tracer := &PostDeltaTracer{
		Sources: make([]map[string]interface{}, 0),
		Errors:  make([]error, 0),
	}
	tracer.PostDeltas = func(entities []string, isAgent bool, deltas ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
		// Check max delta size and simulate submission/return, or error if the payload is too big
		pdb := inventoryapi.PostDeltaBody{ExternalKeys: entities, IsAgent: &isAgent, Deltas: deltas}
		buf, err := json.Marshal(pdb)
		if err != nil {
			tracer.Errors = append(tracer.Errors, err)
			return nil, err
		}
		if len(buf) > maxDeltaSize {
			err = fmt.Errorf("deltas are too big: %v > %v", len(buf), maxDeltaSize)
			tracer.Errors = append(tracer.Errors, err)
			return nil, err
		}

		// Trace delta sources
		sources := map[string]interface{}{}
		for _, d := range deltas {
			sources[d.Source] = 1
		}
		tracer.Sources = append(tracer.Sources, sources)
		return &inventoryapi.PostDeltaResponse{}, nil
	}

	tracer.PostDeltasVortex = func(entityID entity.ID, entities []string, isAgent bool, deltas ...*inventoryapi.RawDelta) (*inventoryapi.PostDeltaResponse, error) {
		// Check max delta size and simulate submission/return, or error if the payload is too big
		pdb := inventoryapi.PostDeltaVortexBody{EntityID: entityID, IsAgent: &isAgent, Deltas: deltas}
		buf, err := json.Marshal(pdb)
		if err != nil {
			tracer.Errors = append(tracer.Errors, err)
			return nil, err
		}
		if len(buf) > maxDeltaSize {
			err = fmt.Errorf("deltas are too big: %v > %v", len(buf), maxDeltaSize)
			tracer.Errors = append(tracer.Errors, err)
			return nil, err
		}

		// Trace delta sources
		sources := map[string]interface{}{}
		for _, d := range deltas {
			sources[d.Source] = 1
		}
		tracer.Sources = append(tracer.Sources, sources)
		return &inventoryapi.PostDeltaResponse{}, nil
	}

	return tracer
}

// GetFreePort asks the kernel for a free open port that is ready to use.
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// util class for Eventually
type testResult struct {
	sync.RWMutex
	failed  bool
	errorCh chan<- error
	failCh  chan<- error
}

func (te *testResult) Errorf(format string, args ...interface{}) {
	te.Lock()
	te.failed = true
	te.Unlock()
	te.errorCh <- fmt.Errorf(format, args...)
}

func (te *testResult) FailNow() {
	te.Lock()
	te.failed = true
	te.Unlock()
	te.failCh <- errors.New("test failed")
}

func (te *testResult) HasFailed() bool {
	te.RLock()
	defer te.RUnlock()
	return te.failed
}

// Eventually retries a test until it eventually succeeds. If the timeout is reached, the test fails
// with the same failure as its last execution.
func Eventually(t *testing.T, timeout time.Duration, testFunc func(_ require.TestingT)) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	success := make(chan interface{})
	errorCh := make(chan error)
	failCh := make(chan error)

	go func() {
		for ctx.Err() == nil {
			result := testResult{failed: false, errorCh: errorCh, failCh: failCh}
			// Executing the function to test
			testFunc(&result)
			// If the function didn't reported failure and didn't reached timeout
			if !result.HasFailed() && ctx.Err() == nil {
				success <- 1
				break
			}
		}
	}()

	// Wait for success or timeout
	var err, fail error
	for {
		select {
		case <-success:
			return
		case err = <-errorCh:
		case fail = <-failCh:
		case <-ctx.Done():
			if err != nil {
				t.Error(err)
			} else if fail != nil {
				t.Error(fail)
			} else {
				t.Error("timeout while waiting for test to complete")
			}
			return
		}
	}
}

// SetupLog sets the log on verbose when test is run with -v flag.
func SetupLog() {
	if testing.Verbose() {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func InventoryDuration(pluginIntervalSecs int64) time.Duration {
	// pluginIntervalSecs might be 0 when config is non initialized
	maximumPluginRunSecs := int64(30)
	return time.Duration(pluginIntervalSecs+maximumPluginRunSecs) * time.Second
}

// Setenv sets an environment variable and returns a function that unsets the variable
// or restores it to its previous value
func Setenv(variable, value string) func() {
	previous, hasPrevious := os.LookupEnv(variable)
	os.Setenv(variable, value)
	return func() {
		if hasPrevious {
			os.Setenv(variable, previous)
		} else {
			os.Unsetenv(variable)
		}
	}
}
