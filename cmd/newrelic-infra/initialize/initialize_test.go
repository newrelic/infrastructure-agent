// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package initialize

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	logHelper "github.com/newrelic/infrastructure-agent/test/log"
)

var (
	errForMkdir = errors.New("this is an error for mkdir")
	errForRmdir = errors.New("this is an error for rmdir")
)

//nolint:tparallel
func Test_emptyTemporaryFolder(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		agentTempDir  string
		removeFunc    func(string) error
		mkdirFunc     func(string, os.FileMode) error
		expectedError error
	}{
		{
			name:         "Empty conf option should not be deleted",
			agentTempDir: "",
		},
		{
			name:         "Some Random Existing Path should not be deleted",
			agentTempDir: "/some/random/path",
		},
		{
			name:         "Default path should be deleted",
			agentTempDir: agentTemporaryFolder,
			removeFunc:   func(string) error { return nil },
			mkdirFunc:    func(string, os.FileMode) error { return nil },
		},
		{
			name:          "Error removing should log",
			agentTempDir:  agentTemporaryFolder,
			removeFunc:    func(string) error { return errForRmdir },
			mkdirFunc:     func(string, os.FileMode) error { return nil },
			expectedError: fmt.Errorf("can't empty agent temporary folder: %w", errForRmdir),
		},
		{
			name:          "Error creating should log",
			agentTempDir:  agentTemporaryFolder,
			removeFunc:    func(string) error { return nil },
			mkdirFunc:     func(string, os.FileMode) error { return errForMkdir },
			expectedError: fmt.Errorf("can't create agent temporary folder: %w", errForMkdir),
		},
		{
			name:          "Error creating and removing should log both",
			agentTempDir:  agentTemporaryFolder,
			removeFunc:    func(string) error { return errForRmdir },
			mkdirFunc:     func(string, os.FileMode) error { return errForMkdir },
			expectedError: fmt.Errorf("can't empty agent temporary folder: %w", errForRmdir),
		},
	}

	defer func() {
		removeFunc = os.RemoveAll
		mkdirFunc = os.MkdirAll
	}()

	//nolint:paralleltest
	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			cfg := &config.Config{
				AgentTempDir: testCase.agentTempDir,
			}

			hook := logHelper.NewInMemoryEntriesHook([]logrus.Level{logrus.FatalLevel, logrus.ErrorLevel})
			log.AddHook(hook)

			mkdirFunc = testCase.mkdirFunc
			removeFunc = testCase.removeFunc
			err := emptyTemporaryFolder(cfg)
			assert.Equal(t, testCase.expectedError, err)
		})
	}
}
