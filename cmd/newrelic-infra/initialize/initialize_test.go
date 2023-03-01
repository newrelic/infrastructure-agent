// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package initialize

import (
	"errors"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	logHelper "github.com/newrelic/infrastructure-agent/test/log"
)

var errForTest = errors.New("this is an error")

func Test_emptyTemporaryFolder(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		agentTempDir       string
		removeFunc         func(string) error
		mkdirFunc          func(string, os.FileMode) error
		expectedLogEntries []string
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
			name:               "Error removing should log",
			agentTempDir:       agentTemporaryFolder,
			removeFunc:         func(string) error { return errForTest },
			mkdirFunc:          func(string, os.FileMode) error { return nil },
			expectedLogEntries: []string{"Can't empty agent temporary folder"},
		},
		{
			name:               "Error creating should log",
			agentTempDir:       agentTemporaryFolder,
			removeFunc:         func(string) error { return nil },
			mkdirFunc:          func(string, os.FileMode) error { return errForTest },
			expectedLogEntries: []string{"Can't create agent temporary folder"},
		},
		{
			name:               "Error creating and removing should log both",
			agentTempDir:       agentTemporaryFolder,
			removeFunc:         func(string) error { return errForTest },
			mkdirFunc:          func(string, os.FileMode) error { return errForTest },
			expectedLogEntries: []string{"Can't empty agent temporary folder", "Can't create agent temporary folder"},
		},
	}

	defer func() {
		removeFunc = os.RemoveAll
		mkdirFunc = os.MkdirAll
	}()
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
			emptyTemporaryFolder(cfg)
			for i, entry := range hook.GetEntries() {
				assert.Equal(t, entry.Message, testCase.expectedLogEntries[i])
			}
		})
	}
}
