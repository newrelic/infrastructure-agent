// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package linux

import (
	"bytes"
	"errors"
	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackageInfo(t *testing.T) {
	p := NewRpmPlugin(testing2.NewMockAgent())
	rpmP := p.(*rpmPlugin)

	var tests = []struct {
		output   string
		packages []RpmItem
		count    int
		err      error
	}{
		{"test\ntwo", []RpmItem{}, 0, nil},
		{"test 1.0 r2 x386 12345 (none)\ntwo", []RpmItem{{"test", "1.0", "r2", "x386", "12345", "none"}}, 1, nil},
		{"test 1.0 r2 x386 12345 9\ntwo", []RpmItem{{"test", "1.0", "r2", "x386", "12345", "9"}}, 1, nil},
		{"test 1.0 r2 x386 12345 9\nchuck 1.9 r3 x386 92345 myepo", []RpmItem{{"chuck", "1.9", "r3", "x386", "92345", "myepo"}, {"test", "1.0", "r2", "x386", "12345", "9"}}, 2, nil},
		{"test 1.0 r2 x386 12345 9\ntest 1.9 r3 x386 92345 myepo", []RpmItem{{"test", "1.0", "r2", "x386", "12345", "9"}, {"test-1", "1.9", "r3", "x386", "92345", "myepo"}}, 2, nil},
	}

	for _, test := range tests {
		packages, err := rpmP.parsePackageInfo(test.output)
		assert.Len(t, packages, test.count)
		sort.Sort(packages)
		for i, result := range packages {
			assert.Equal(t, test.packages[i], result)
		}
		assert.Equal(t, test.err, err)
	}
}

func TestParsePackageInfo_WarnsOnDroppedLines(t *testing.T) {
	var w bytes.Buffer
	log.SetOutput(&w)

	p := NewRpmPlugin(testing2.NewMockAgent())
	rpmP := p.(*rpmPlugin)
	resultPackages, err := rpmP.parsePackageInfo("test 1.0 r2 x386 12345 (none)\nfoo")
	assert.NoError(t, err)

	assert.Contains(t, w.String(), "cannot parse rpm query line")
	assert.Contains(t, w.String(), "foo")

	require.Len(t, resultPackages, 1)
	assert.Equal(t, RpmItem{"test", "1.0", "r2", "x386", "12345", "none"}, resultPackages[0])
}

func TestParsePackageInfo_WarnsOnDroppedLinesOncePerLine(t *testing.T) {
	var w bytes.Buffer
	log.SetOutput(&w)

	p := NewRpmPlugin(testing2.NewMockAgent())
	rpmP := p.(*rpmPlugin)
	resultPackages, err := rpmP.parsePackageInfo("foo\ntest 1.0 r2 x386 12345 (none)\nbar\nfoo")
	assert.NoError(t, err)

	assert.Equal(t, 1, strings.Count(w.String(), "foo"))
	assert.Equal(t, 1, strings.Count(w.String(), "bar"))

	require.Len(t, resultPackages, 1)
	assert.Equal(t, RpmItem{"test", "1.0", "r2", "x386", "12345", "none"}, resultPackages[0])
}

// fakeFileWatcher accepts only the paths listed in watchable and records every path it was asked to watch.
type fakeFileWatcher struct {
	watchable map[string]struct{}
	attempts  []string
}

func (f *fakeFileWatcher) Add(name string) error {
	f.attempts = append(f.attempts, name)
	if _, ok := f.watchable[name]; ok {
		return nil
	}

	return errors.New("no such file or directory")
}

func TestWatchFirstAvailable(t *testing.T) {
	t.Parallel()

	paths := []string{"/db/first", "/db/second", "/db/third"}

	cases := []struct {
		name         string
		watchable    []string
		wantErr      bool
		wantAttempts []string
	}{
		{
			name:         "first path available",
			watchable:    []string{"/db/first", "/db/second", "/db/third"},
			wantAttempts: []string{"/db/first"},
		},
		{
			name:         "falls back to second path",
			watchable:    []string{"/db/second", "/db/third"},
			wantAttempts: []string{"/db/first", "/db/second"},
		},
		{
			name:         "falls back to last path",
			watchable:    []string{"/db/third"},
			wantAttempts: []string{"/db/first", "/db/second", "/db/third"},
		},
		{
			name:         "no path available",
			wantErr:      true,
			wantAttempts: []string{"/db/first", "/db/second", "/db/third"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			watcher := &fakeFileWatcher{watchable: map[string]struct{}{}, attempts: nil}
			for _, path := range tc.watchable {
				watcher.watchable[path] = struct{}{}
			}

			err := watchFirstAvailable(watcher, paths)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantAttempts, watcher.attempts)
		})
	}
}

func TestRpmDBWatchPaths_SupportsAllRpmBackends(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{
		"/var/lib/rpm/Installtid",
		"/var/lib/rpm/rpmdb.sqlite",
		"/var/lib/rpm/Packages.db",
	}, rpmDBWatchPaths)
}

// Only the ndb backend file exists, as on SLES 16.0: the watcher must be set up and report writes to it.
func TestWatchFirstAvailable_NdbBackendWithRealWatcher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ndbFile := filepath.Join(dir, "Packages.db")
	require.NoError(t, os.WriteFile(ndbFile, []byte("initial"), 0o600))

	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer func() { _ = watcher.Close() }()

	paths := []string{
		filepath.Join(dir, "Installtid"),
		filepath.Join(dir, "rpmdb.sqlite"),
		ndbFile,
	}
	require.NoError(t, watchFirstAvailable(watcher, paths))

	require.NoError(t, os.WriteFile(ndbFile, []byte("package installed"), 0o600))

	select {
	case event := <-watcher.Events:
		assert.Equal(t, ndbFile, event.Name)
		assert.True(t, event.Op&fsnotify.Write == fsnotify.Write)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for the write event on the ndb database file")
	}
}
