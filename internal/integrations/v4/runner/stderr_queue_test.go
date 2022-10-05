// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package runner

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestQueueSizeConfig(t *testing.T) {
	testCases := []struct {
		name         string
		expected     string
		noOfLogLines int
		queueSize    int
	}{
		{
			name:         "Disabled",
			queueSize:    -1,
			expected:     "(no standard error output)",
			noOfLogLines: 10,
		},
		{
			name:         "Default",
			queueSize:    0,
			expected:     `(last 10 lines out of 20): log_line:11\nlog_line:12\nlog_line:13\nlog_line:14\nlog_line:15\nlog_line:16\nlog_line:17\nlog_line:18\nlog_line:19\nlog_line:20`,
			noOfLogLines: 20,
		},
		{
			name:         "QueueNotFull",
			queueSize:    10,
			expected:     `log_line:1\nlog_line:2\nlog_line:3\nlog_line:4\nlog_line:5\nlog_line:6\nlog_line:7\nlog_line:8\nlog_line:9`,
			noOfLogLines: 9,
		},
		{
			name:         "CustomSize",
			queueSize:    3,
			expected:     `(last 3 lines out of 9): log_line:7\nlog_line:8\nlog_line:9`,
			noOfLogLines: 9,
		},
		{
			name:         "NoLines",
			queueSize:    3,
			expected:     `(no standard error output)`,
			noOfLogLines: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			queue := newStderrQueue(testCase.queueSize)

			for i := 1; i <= testCase.noOfLogLines; i++ {
				queue.Add([]byte(fmt.Sprintf("log_line:%d", i)))
			}

			actual := strings.ReplaceAll(queue.Flush(), "\n", "\\n")
			assert.Equal(t, testCase.expected, actual)
		})
	}
}
