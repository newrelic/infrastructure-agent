// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package delta

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLastSubmissionStore(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	repoDir := filepath.Join(dataDir, "delta")
	ds := NewLastSubmissionStore(repoDir)

	now := time.Now()
	assert.NoError(t, ds.UpdateTime(now))

	got, err := ds.Time()
	assert.NoError(t, err)

	assert.Equal(t, now, got)
}

func TestStore_LastSuccessSubmission_ReadsFromDiskWhenNoInMemryData(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	repoDir := filepath.Join(dataDir, "delta")
	ls := NewLastSubmissionStore(repoDir).(*LastSubmissionFileStore)

	now := time.Now()
	assert.NoError(t, ls.UpdateTime(now))
	ls.t = time.Time{} // empty in memory value

	got, err := ls.Time() // read
	assert.NoError(t, err)

	assert.Equal(t, now.Unix(), got.Unix())
}

func TestStore_LastSuccessSubmission_MemoryFirst(t *testing.T) {
	dataDir, err := TempDeltaStoreDir()
	assert.NoError(t, err)
	repoDir := filepath.Join(dataDir, "delta")
	ds := NewLastSubmissionStore(repoDir).(*LastSubmissionFileStore)

	aprilFirst, err := time.Parse(time.RFC3339, "2020-04-01T00:00:00+00:00")
	mayFirst, err := time.Parse(time.RFC3339, "2020-05-01T00:00:00+00:00")
	ds.updateLastSuccessSubmission(aprilFirst)
	err = ds.saveLastSuccessSubmission()
	assert.NoError(t, err)
	ds.updateLastSuccessSubmission(mayFirst)

	actual, err := ds.Time()
	assert.NoError(t, err)

	assert.EqualValues(t, mayFirst.Unix(), actual.Unix())
}
