// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatTime(t *testing.T) {
	date := time.Date(2022, time.January, 1, 01, 23, 45, 0, time.Local)

	testCases := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "TokensAreReplaced",
			pattern:  "YYYY/MM/DD-hh:mm:ss",
			expected: "2022/01/01-01:23:45",
		},
		{
			name:     "MultipleReplacements",
			pattern:  "YYYY YYYY/MM MM/DD DD-hh hh:mm mm:ss ss",
			expected: "2022 2022/01 01/01 01-01 01:23 23:45 45",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			formattedTime := formatTime(testCase.pattern, date)
			assert.Equal(t, testCase.expected, formattedTime)
		})
	}
}

func TestGenerateFileName(t *testing.T) {
	date := time.Date(2022, time.January, 1, 01, 23, 45, 0, time.Local)

	testCases := []struct {
		name     string
		config   FileWithRotationConfig
		expected string
	}{
		{
			name: "FileWithDate",
			config: FileWithRotationConfig{
				File: "newrelic-infra.log",
			},
			expected: "newrelic-infra_2022-01-01_01-23-45.log",
		},
		{
			name: "FileWithPathAndDate",
			config: FileWithRotationConfig{
				File: "/var/log/newrelic-infra/newrelic-infra.log",
			},
			expected: "newrelic-infra_2022-01-01_01-23-45.log",
		},
		{
			name: "FileWithTokensInPath",
			config: FileWithRotationConfig{
				File: "/var/log/newrelic-infraYYYYMMDDhhmmss/newrelic-infra.log",
			},
			expected: "newrelic-infra_2022-01-01_01-23-45.log",
		},
		{
			name: "FileWithTokensInExtension",
			config: FileWithRotationConfig{
				File: "/var/log/newrelic-infra/newrelic-infra.logYYYYMMDDhhmmss",
			},
			expected: "newrelic-infra_2022-01-01_01-23-45.log20220101012345",
		},
		{
			name: "CustomPattern",
			config: FileWithRotationConfig{
				File:            "/var/log/newrelic-infra/newrelic-infra.log",
				FileNamePattern: "xyz_YYYY:DD:MM:hh:mm:ss",
			},
			expected: "xyz_2022:01:01:01:23:45",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := FileWithRotation{
				cfg: testCase.config,
				getTimeFn: func() time.Time {
					return date
				},
			}
			fileName := formatTime(f.generateFileName(), date)
			assert.Equal(t, testCase.expected, fileName)
		})
	}
}

func TestOpenFileWithRotation(t *testing.T) {
	logFile := filepath.Join(os.TempDir(), "newrelic-infra.log")
	cfg := FileWithRotationConfig{
		File:            logFile,
		FileNamePattern: "",
	}

	// GIVEN a new NewFileWithRotation
	file, err := NewFileWithRotation(cfg).Open()

	defer func() {
		assert.NoError(t, file.Close())
		assert.NoError(t, os.Remove(logFile))
	}()

	require.NoError(t, err)

	// File can be opened
	_, err = os.Stat(logFile)
	require.NoError(t, err)
}

func TestNewContentFitsMaxSizeInBytes(t *testing.T) {
	logFile := filepath.Join(os.TempDir(), "newrelic-infra.log")

	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  1,
		FileNamePattern: "",
	}

	file, err := NewFileWithRotation(cfg).Open()

	defer func() {
		assert.NoError(t, file.Close())
		assert.NoError(t, os.Remove(logFile))
	}()

	require.NoError(t, err)

	// WHEN writing a content that exceeds the maxSize config
	n, err := file.Write([]byte{1, 2})

	// THEN error is returned
	assert.Equal(t, n, 0)
	assert.EqualError(t, err, "failed to write to file, new content size: '2b' exceeds to maximum file size: '1b'")

	// WHEN writing a message that fits the maxSize
	n, err = file.Write([]byte{1})

	// THEN no error is returned
	assert.Equal(t, n, 1)
	assert.NoError(t, err)
}

func TestFileRotate(t *testing.T) {
	tmp := os.TempDir()
	logFile := filepath.Join(tmp, "newrelic-infra.log")
	rotatedLogFile := filepath.Join(tmp, "newrelic-infra_2022-01-01_01-23-45.log")

	// Make sure files don't exist.
	os.Remove(logFile)
	os.Remove(rotatedLogFile)

	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  1,
		FileNamePattern: "",
	}

	file, err := NewFileWithRotation(cfg).Open()

	defer func() {
		assert.NoError(t, file.Close())

		assert.NoError(t, os.Remove(logFile))
		assert.NoError(t, os.Remove(rotatedLogFile))
	}()

	require.NoError(t, err)

	// Mock the date for filename rename
	file.getTimeFn = func() time.Time {
		return time.Date(2022, time.January, 1, 01, 23, 45, 0, time.Local)
	}

	content := []byte{1}

	// WHEN writing a message that doesn't exceed the limit
	n, err := file.Write(content)

	// THEN no error is returned
	assert.Equal(t, n, 1)
	assert.NoError(t, err)

	// WHEN writing another message when max size already reached
	n, err = file.Write(content)

	// THEN no error is returned
	assert.Equal(t, n, 1)
	assert.NoError(t, err)

	// AND file was rotated
	_, err = os.Stat(rotatedLogFile)
	require.NoError(t, err)

	// AND content from both file matches expectations
	b, err := ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, content, b)

	b, err = ioutil.ReadFile(rotatedLogFile)
	assert.NoError(t, err)
	assert.Equal(t, content, b)

	// AND written bytes counter was reset.
	assert.Equal(t, file.writtenBytes, int64(1))
}

func TestCloseAlreadyClosedFile(t *testing.T) {
	tmp := os.TempDir()
	logFile := filepath.Join(tmp, "newrelic-infra.log")

	// Make sure files don't exist.
	os.Remove(logFile)

	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  1,
		FileNamePattern: "",
	}

	file, err := NewFileWithRotation(cfg).Open()

	defer func() {
		assert.NoError(t, os.Remove(logFile))
	}()

	require.NoError(t, err)

	// THEN no error on 1st close call
	err = file.Close()
	assert.NoError(t, err)

	// AND error is returned on 2nd close call
	err = file.Close()
	assert.Error(t, err)
}

func TestWrite(t *testing.T) {
	tmp := os.TempDir()
	logFile := filepath.Join(tmp, "newrelic-infra.log")

	// Make sure files don't exist.
	os.Remove(logFile)

	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  100,
		FileNamePattern: "",
	}

	file, err := NewFileWithRotation(cfg).Open()

	defer func() {
		assert.NoError(t, file.Close())
		assert.NoError(t, os.Remove(logFile))
	}()

	require.NoError(t, err)

	// WHEN writing a message
	written1, err := file.Write([]byte("message1"))

	// THEN no error
	require.NoError(t, err)

	// WHEN closing the file and writing a message
	err = file.Close()
	require.NoError(t, err)

	written, err := file.Write([]byte("something else"))

	// THEN error is returned
	require.Equal(t, written, 0)
	require.Error(t, err)

	// WHEN reopening the file
	_, err = file.Open()
	require.NoError(t, err)

	// THEN no error is returned
	written2, err := file.Write([]byte("message2"))
	require.NoError(t, err)

	// AND writtenBytes is increased
	assert.Equal(t, file.writtenBytes, int64(written1+written2))

	// AND the content matches the expectation
	b, err := ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, []byte("message1message2"), b)
}

func TestWhenFileNotOpen(t *testing.T) {
	tmp := os.TempDir()
	logFile := filepath.Join(tmp, "newrelic-infra.log")

	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  100,
		FileNamePattern: "",
	}

	file := NewFileWithRotation(cfg)

	// WHEN writing to a file that is not opened
	n, err := file.Write([]byte("message1"))

	// THEN error is returned
	assert.Equal(t, n, 0)
	require.ErrorIs(t, err, ErrFileNotOpened)

	// WHEN closing the file
	err = file.Close()

	// Error is returned
	require.ErrorIs(t, err, ErrFileNotOpened)
}

func TestFailToRotateDoesntPreventLogging(t *testing.T) {
	tmp := os.TempDir()
	logFile := filepath.Join(tmp, "newrelic-infra.log")

	maxBytes := 2
	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:           logFile,
		MaxSizeInBytes: int64(maxBytes),
		// Use Illegal Filename Character . to trigger rotate error.
		FileNamePattern: ".",
	}

	file, err := NewFileWithRotation(cfg).Open()

	defer func() {
		assert.NoError(t, file.Close())
		assert.NoError(t, os.Remove(logFile))
	}()

	assert.NoError(t, err)

	// WHEN maxBytes is exceeded and rotate fails.
	bytesToWrite := maxBytes * 5
	for i := 0; i < bytesToWrite; i++ {
		written, err := file.Write([]byte{byte(i + int('a'))})

		// THEN no error occurred
		assert.Equal(t, written, 1)
		assert.NoError(t, err)
	}

	// AND no content is lost
	assert.Equal(t, file.writtenBytes, int64(bytesToWrite))

	b, err := ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, "abcdefghij", string(b))
}
