// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchReapNoCacheExists(t *testing.T) {
	fixture, err := ioutil.ReadFile(filepath.Join("fixtures", "packages_rpm_delta.json"))
	if err != nil {
		t.Fatal(err)
		return
	}

	mockFiles := []testhelpers.MockFile{{
		Name:      "rpm.json",
		ParentDir: filepath.Join("packages", "__nria_localentity"),
		Content:   string(fixture),
	}}

	dir, err := testhelpers.NewMockDir(mockFiles)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer dir.Clear()

	store := delta.NewStore(dir.Path, "default", maxInventoryDataSize)
	pr := newPatchReaper("", store)
	pr.Reap()
	cacheFilePath := filepath.Join(
		dir.Path,
		".delta_repo",
		"packages",
		"__nria_localentity",
		"rpm.json",
	)

	require.FileExists(t, cacheFilePath)
	cacheB, err := ioutil.ReadFile(cacheFilePath)
	assert.Equal(t, fixture, cacheB)
}

func TestPatchReapCacheUpdate(t *testing.T) {
	sourceF, err := ioutil.ReadFile(filepath.Join("fixtures", "packages_rpm_delta.json"))
	if err != nil {
		t.Fatal(err)
		return
	}
	cacheF, err := ioutil.ReadFile(filepath.Join("fixtures", "packages_rpm_delta_2.json"))
	if err != nil {
		t.Fatal(err)
		return
	}

	mockFiles := []testhelpers.MockFile{
		{
			Name:      "rpm.json",
			ParentDir: filepath.Join("packages", "__nria_localentity"),
			Content:   string(sourceF),
		},
		{
			Name:      "rpm.json",
			ParentDir: filepath.Join(".delta_repo", "packages", "__nria_localentity"),
			Content:   string(cacheF),
		},
	}

	dir, err := testhelpers.NewMockDir(mockFiles)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer dir.Clear()

	store := delta.NewStore(dir.Path, "default", maxInventoryDataSize)
	pr := newPatchReaper("", store)
	pr.Reap()
	cacheFilePath := filepath.Join(
		dir.Path,
		".delta_repo",
		"packages",
		"__nria_localentity",
		"rpm.json",
	)

	cacheB, err := ioutil.ReadFile(cacheFilePath)
	assert.Equal(t, sourceF, cacheB)
}

func TestPatchReapEmptyDelta(t *testing.T) {
	fixtureB, err := ioutil.ReadFile(filepath.Join("fixtures", "packages_rpm_delta.json"))
	if err != nil {
		t.Fatal(err)
		return
	}
	fixture := string(fixtureB)
	mockFiles := []testhelpers.MockFile{
		{
			Name:      "rpm.json",
			ParentDir: filepath.Join("packages", "__nria_localentity"),
			Content:   fixture,
		},
		{
			Name:      "rpm.json",
			ParentDir: filepath.Join(".delta_repo", "packages", "__nria_localentity"),
			Content:   fixture,
		},
	}

	dir, err := testhelpers.NewMockDir(mockFiles)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer dir.Clear()

	cacheFilePath := filepath.Join(
		dir.Path,
		".delta_repo",
		"packages",
		"__nria_localentity",
		"rpm.json",
	)
	s1, err := os.Stat(cacheFilePath)

	store := delta.NewStore(dir.Path, "default", maxInventoryDataSize)
	pr := newPatchReaper("", store)
	pr.Reap()

	s2, err := os.Stat(cacheFilePath)
	assert.Equal(t, s1.ModTime(), s2.ModTime())

	cacheB, err := ioutil.ReadFile(cacheFilePath)
	assert.Equal(t, fixtureB, cacheB)
}

func BenchmarkPatchReapNoDiff(b *testing.B) {

	fixB, err := ioutil.ReadFile(filepath.Join("fixtures", "packages_rpm_delta.json"))
	if err != nil {
		b.Fatal(err)
		return
	}

	fixture := string(fixB)
	mockFiles := []testhelpers.MockFile{
		{
			Name:      "rpm.json",
			ParentDir: filepath.Join("packages", "__nria_localentity"),
			Content:   fixture,
		},
		{
			Name:      "rpm.json",
			ParentDir: filepath.Join(".delta_repo", "packages", "__nria_localentity"),
			Content:   fixture,
		},
	}

	dir, err := testhelpers.NewMockDir(mockFiles)
	if err != nil {
		b.Fatal(err)
		return
	}
	defer dir.Clear()

	store := delta.NewStore(dir.Path, "default", maxInventoryDataSize)
	pr := newPatchReaper("", store)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pr.Reap()
	}
}

func BenchmarkPatchReapWithDiff(b *testing.B) {

	b.StopTimer()
	b.ResetTimer()

	fixture, err := ioutil.ReadFile(filepath.Join("fixtures", "packages_rpm_delta.json"))
	if err != nil {
		b.Fatal(err)
		return
	}
	fixture2B, err := ioutil.ReadFile(filepath.Join("fixtures", "packages_rpm_delta_2.json"))
	if err != nil {
		b.Fatal(err)
		return
	}
	fixture2 := string(fixture2B)

	mockFiles := []testhelpers.MockFile{
		{
			Name:      "rpm.json",
			ParentDir: filepath.Join("packages", "__nria_localentity"),
			Content:   string(fixture),
		},
	}

	dir, err := testhelpers.NewMockDir(mockFiles)
	if err != nil {
		b.Fatal(err)
		return
	}
	defer dir.Clear()

	store := delta.NewStore(dir.Path, "default", maxInventoryDataSize)
	pr := newPatchReaper("", store)
	var mockSource testhelpers.MockFile
	for i := 0; i < b.N; i++ {
		mockSource = testhelpers.MockFile{
			Name:      "rpm.json",
			ParentDir: filepath.Join(".delta_repo", "packages", "__nria_localentity"),
			Content:   fixture2,
		}
		if err = dir.AddFile(mockSource); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		pr.Reap()
		b.StopTimer()
	}
}
