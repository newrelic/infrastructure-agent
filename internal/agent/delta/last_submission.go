// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package delta

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// LastSubmissionStore retrieves and stores last inventory successful submission time.
type LastSubmissionStore interface {
	Time() (time.Time, error)
	UpdateTime(time.Time) error
}

// LastSubmissionFileStore persists last successful submission date.
type LastSubmissionFileStore struct {
	// file holds the file path storing the data
	file string
	// t time of last successful inventory submission
	t time.Time
}

// NewLastSubmissionStore creates a new LastSubmissionStore storing data in file.
func NewLastSubmissionStore(dataDir string) LastSubmissionStore {
	return &LastSubmissionFileStore{
		file: filepath.Join(dataDir, lastSuccessSubmissionFile),
	}
}

func (l *LastSubmissionFileStore) Time() (time.Time, error) {

	if !l.t.Equal(time.Time{}) {
		return l.t, nil
	}

	content, err := ioutil.ReadFile(l.file)

	if err != nil {
		return time.Time{}, err
	}

	if err = l.t.UnmarshalText(content); err != nil {
		return time.Time{}, err
	}

	if !l.t.Equal(time.Time{}) {
		return l.t, nil
	}

	return time.Time{}, ErrNoPreviousSuccessSubmissionTime
}

func (l *LastSubmissionFileStore) UpdateTime(t time.Time) error {
	l.updateLastSuccessSubmission(t)
	return l.saveLastSuccessSubmission()
}

func (l *LastSubmissionFileStore) updateLastSuccessSubmission(submissionTime time.Time) {
	l.t = submissionTime
}

func (l *LastSubmissionFileStore) saveLastSuccessSubmission() error {
	serialised, err := l.t.MarshalText()

	if err != nil {
		return err
	}

	dir := filepath.Dir(l.file)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, DATA_DIR_MODE); err != nil {
			return fmt.Errorf("submission store directory does not exist and cannot be created: %s", dir)
		}
	}

	if err = ioutil.WriteFile(l.file, serialised, DATA_DIR_MODE); err != nil {
		return err
	}

	return nil
}
