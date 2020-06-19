// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package linux

import (
	"bytes"
	testing2 "github.com/newrelic/infrastructure-agent/internal/plugins/testing"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"sort"
	"strings"
	"testing"

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
