// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package delta

import (
	"time"
)

// LastSubmissionInMemory stores last successful submission date in memory.
type LastSubmissionInMemory struct {
	// t time of last successful inventory submission
	t time.Time
}

// NewLastSubmissionInMemory creates a fake LastSubmissionStore with ephemeral persistence for tests
// new instance assumes current time as last submission.
func NewLastSubmissionInMemory() LastSubmissionStore {
	return &LastSubmissionInMemory{
		t: time.Now(),
	}
}

func (l *LastSubmissionInMemory) Time() (time.Time, error) {
	return l.t, nil
}

func (l *LastSubmissionInMemory) UpdateTime(t time.Time) error {
	l.t = t
	return nil
}
