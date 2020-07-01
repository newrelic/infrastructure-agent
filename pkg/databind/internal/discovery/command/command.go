// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/naming"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

var (
	timeoutError = errors.New("command timed out")
)

// Discoverer returns an executable discoverer from the provided configuration.
// The fetching process will return an array of map values
func Discoverer(d discovery.Command) (fetchDiscoveries func() (discoveries []discovery.Discovery, err error), err error) {
	matcher, err := discovery.NewMatcher(d.Matcher)
	if err != nil {
		return nil, err
	}
	cmd := newCommand(d, matcher)
	return cmd.fetch, err
}

func newCommand(d discovery.Command, matcher discovery.FieldsMatcher) (exe *executable) {
	return &executable{d: d, m: matcher, run: run}
}

type executable struct {
	d   discovery.Command
	m   discovery.FieldsMatcher
	run func(d discovery.Command) (results []data.GenericDiscovery, err error)
}

func run(d discovery.Command) (results []data.GenericDiscovery, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, d.Exec[0], d.Exec[1:]...)
	cmd.Env = os.Environ()
	for k := range d.Environment {
		cmd.Env = append(cmd.Env, k+"="+d.Environment[k])
	}

	done := make(chan error)
	go func() {
		defer close(done)
		out := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.Stdout = out
		cmd.Stderr = stderr
		decoder := json.NewDecoder(out)
		e := cmd.Run()
		if e != nil {
			done <- errors.New(stderr.String() + e.Error())
		}
		done <- decoder.Decode(&results)
	}()

	timeout := time.Minute
	if d.Timeout > 0 {
		timeout = d.Timeout
	}
	select {
	case <-time.After(timeout):
		cancel()
		return results, timeoutError
	case err = <-done:
		return results, err
	}
}

func (e *executable) fetch() (results []discovery.Discovery, err error) {

	res, err := e.run(e.d)
	if err != nil {
		return results, err
	}

	for resI := range res {
		fields := data.InterfaceMapToMap(res[resI].Variables)
		if e.m.All(fields) {
			results = append(results, discovery.Discovery{
				Variables:         discovery.LabelsToMap(naming.DiscoveryPrefix, fields),
				MetricAnnotations: res[resI].Annotations,
				EntityRewrites:    res[resI].EntityRewrites,
			})
		}
	}
	return results, err
}
