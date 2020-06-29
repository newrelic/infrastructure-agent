// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/naming"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestDiscoverer(t *testing.T) {
	t.Parallel()
	f, err := ioutil.TempFile("", "TestDiscoverer")
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	d := discovery.Command{
		Exec: discovery.ShlexOpt{f.Name(), "expected", "args"},
	}
	fn, err := Discoverer(d)
	require.NoError(t, err)
	require.NotNil(t, fn)
}

func TestDiscoverer_bad_matcher(t *testing.T) {
	t.Parallel()
	d := discovery.Command{
		Exec: discovery.ShlexOpt{path.Join("path", "that", "does", "not", "exist")},
		Matcher: map[string]string{
			"something": "/BBB(((?!BBB).)*)EEE/",
		},
	}
	fn, err := Discoverer(d)
	require.Error(t, err)
	require.Nil(t, fn)
	assert.EqualError(t, err, "value of \"something\" should be a valid regular expression: error parsing regexp: invalid or unsupported Perl syntax: `(?!`")
}

func TestFetch(t *testing.T) {
	//t.Parallel()
	tests := []struct {
		name         string
		runExeValues []data.GenericDiscovery
		runErr       error
		matcher      map[string]string
		expCount     int
		exptItem     int
	}{
		{
			name:     "happy",
			expCount: 1,
			exptItem: 0,
			runExeValues: []data.GenericDiscovery{
				{
					Variables: data.InterfaceMap{
						"ip":                "1.2.3.4",
						"port":              1337,
						"version":           2.1,
						"unique_identifier": "123e4567e89b12d3a456426655440000",
					},
					Annotations: data.InterfaceMap{
						"role":        "load_balancer",
						"environment": "production",
					},
					EntityRewrites: []data.EntityRewrite{
						{
							Action:       "replace",
							Match:        "${ip}",
							ReplaceField: "container:${containerId}",
						},
					},
				},
			},
		},
		{
			name:     "matcher",
			expCount: 1,
			exptItem: 1,
			runExeValues: []data.GenericDiscovery{
				{
					Variables: data.InterfaceMap{
						"ip":      "1.2.3.5",
						"port":    101,
						"version": 3,
						"info":    "should not match",
					},
					Annotations: data.InterfaceMap{
						"environment": "production",
					},
				},
				{
					Variables: data.InterfaceMap{
						"ip":      "1.2.3.4",
						"port":    1337,
						"version": 2.1,
					},
					Annotations: data.InterfaceMap{
						"environment": "test",
					},
				},
			},
			matcher: map[string]string{
				"version": "/2\\.*/",
			},
		},
		{
			name:   "sad",
			runErr: errors.New("some error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ioutil.TempFile("", tt.name)
			require.NoError(t, err)
			defer func() {
				_ = f.Close()
				_ = os.Remove(f.Name())
			}()
			d := discovery.Command{
				Exec: discovery.ShlexOpt{f.Name(), "expected", "args"},
			}
			matcher, err := discovery.NewMatcher(tt.matcher)
			require.NoError(t, err)
			exe := newCommand(d, matcher)
			assert.Equal(t, d.Exec, exe.d.Exec)
			exe.run = func(d discovery.Command) (results []data.GenericDiscovery, err error) {
				return tt.runExeValues, tt.runErr
			}

			results, err := exe.fetch()
			if tt.runErr != nil {
				require.EqualError(t, err, tt.runErr.Error())
				return
			} else {
				require.NoError(t, err)
			}
			t.Log(results)
			assert.NotEmpty(t, results)
			assert.Len(t, results, tt.expCount)
			expect := tt.runExeValues[tt.exptItem]

			// Check all variables
			for k := range expect.Variables {
				t.Logf("Variable:%s:%v", k, expect.Variables[k])
				assert.Equal(t, fmt.Sprintf("%v", expect.Variables[k]), results[0].Variables[naming.DiscoveryPrefix+k])
			}

			assert.Equal(t, expect.Annotations, results[0].MetricAnnotations)

			t.Log("Running entity check")
			assert.Equal(t, expect.EntityRewrites, results[0].EntityRewrites)
		})
	}
}
