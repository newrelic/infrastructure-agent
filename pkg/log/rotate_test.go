// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatTime(t *testing.T) {
	date := time.Date(2022, time.January, 1, 10, 23, 45, 0, time.Local)

	testCases := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "TokensAreReplaced",
			pattern:  "YYYY/MM/DD-hh:mm:ss",
			expected: "2022/01/01-10:23:45",
		},
		{
			name:     "MultipleReplacements",
			pattern:  "YYYY YYYY/MM MM/DD DD-hh hh:mm mm:ss ss",
			expected: "2022 2022/01 01/01 01-10 10:23 23:45 45",
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
	date := time.Date(2022, time.January, 1, 10, 23, 45, 0, time.Local)

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
			expected: "newrelic-infra_2022-01-01_10-23-45.log",
		},
		{
			name: "FileWithPathAndDate",
			config: FileWithRotationConfig{
				File: "/var/log/newrelic-infra/newrelic-infra.log",
			},
			expected: "newrelic-infra_2022-01-01_10-23-45.log",
		},
		{
			name: "FileWithTokensInPath",
			config: FileWithRotationConfig{
				File: "/var/log/newrelic-infraYYYYMMDDhhmmss/newrelic-infra.log",
			},
			expected: "newrelic-infra_2022-01-01_10-23-45.log",
		},
		{
			name: "FileWithTokensInExtension",
			config: FileWithRotationConfig{
				File: "/var/log/newrelic-infra/newrelic-infra.logYYYYMMDDhhmmss",
			},
			expected: "newrelic-infra_2022-01-01_10-23-45.log20220101102345",
		},
		{
			name: "CustomPattern",
			config: FileWithRotationConfig{
				File:            "/var/log/newrelic-infra/newrelic-infra.log",
				FileNamePattern: "xyz_YYYY:DD:MM:hh:mm:ss",
			},
			expected: "xyz_2022:01:01:10:23:45",
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

	assert.NoError(t, err)

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

	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, file.Close())
		assert.NoError(t, os.Remove(logFile))
	}()

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

	rotatedLogFile := filepath.Join(tmp, "newrelic-infra_2022-01-01_10-23-45.log")

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

	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, file.Close())

		assert.NoError(t, os.Remove(logFile))
		assert.NoError(t, os.Remove(rotatedLogFile))
	}()

	// Mock the date for filename rename
	file.getTimeFn = func() time.Time {
		return time.Date(2022, time.January, 1, 10, 23, 45, 0, time.Local)
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

	assert.NoError(t, err)

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

	assert.NoError(t, err)

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

	assert.NoError(t, err)

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

func TestCompress(t *testing.T) {
	tmp := os.TempDir()
	logFile := filepath.Join(tmp, "newrelic-infra.log")

	rotatedLogFile := filepath.Join(tmp, "rotated.log")
	compressedFile := fmt.Sprintf("%s.%s", rotatedLogFile, compressedFileExt)

	// Make sure files don't exist.
	os.Remove(logFile)
	os.Remove(rotatedLogFile)
	os.Remove(compressedFile)

	mb10 := 1024 * 1024 * 10
	content := strings.Repeat("1", mb10)

	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  int64(mb10),
		FileNamePattern: filepath.Base(rotatedLogFile),
		Compress:        true,
	}

	file, err := NewFileWithRotation(cfg).Open()
	assert.NoError(t, err)

	defer func() {
		os.Remove(logFile)
		os.Remove(rotatedLogFile)
		os.Remove(compressedFile)
	}()

	// GIVEN a file with 10 mb content
	writtenBytes, err := file.Write([]byte(content))
	require.NoError(t, err)
	require.Equal(t, writtenBytes, mb10)

	// Write an extra byte to trigger file rotate
	writtenBytes, err = file.Write([]byte("1"))
	require.NoError(t, err)
	require.Equal(t, writtenBytes, 1)

	// Wait async gzip to finish
	require.Eventually(t, func() bool {
		// When the file to rotate doesn't exist anymore it means the gzip was created.
		_, statErr := os.Stat(rotatedLogFile)

		return os.IsNotExist(statErr)
	}, 60*time.Second, 100*time.Millisecond, "compressed file not created")

	// THEN the resulted file is valid and contains expected data
	resultFile, err := os.Open(compressedFile)
	require.NoError(t, err)

	gzFileStat, err := resultFile.Stat()
	require.NoError(t, err)

	// Check the size of the .gz file to be less than 1 mb.
	fileSizeInMb := float64(gzFileStat.Size()) / float64(mb10)
	assert.True(t, fileSizeInMb < 1)

	var resultReader io.ReadCloser

	if runtime.GOOS == "windows" {
		var zipReader *zip.ReadCloser

		zipReader, err = zip.OpenReader(resultFile.Name())
		assert.NoError(t, err)
		assert.Len(t, zipReader.File, 1)

		resultReader, err = zipReader.File[0].Open()
		assert.NoError(t, err)
	} else {
		resultReader, err = gzip.NewReader(resultFile)
	}

	defer func() {
		assert.NoError(t, resultReader.Close())
	}()

	assert.NoError(t, err)

	resultContent, err := ioutil.ReadAll(resultReader)

	assert.NoError(t, err)
	assert.Equal(t, content, string(resultContent))
}

func TestCompressMemoryUsage(t *testing.T) {
	tmp := os.TempDir()
	logFile := filepath.Join(tmp, "newrelic-infra.log")

	// Make sure files don't exist.
	os.Remove(logFile)

	// GIVEN a file with 300 mb content
	file, err := disk.OpenFile(logFile, os.O_RDWR|os.O_CREATE, filePerm)

	defer func() {
		os.Remove(logFile)
		os.Remove(logFile + ".gz")
	}()

	require.NoError(t, err)

	mb300 := 1024 * 1024 * 300
	content := strings.Repeat("1", mb300)
	_, err = file.Write([]byte(content))
	require.NoError(t, err)

	// WHEN compressing the file using buffered reader/writer
	var baseline, after runtime.MemStats

	runtime.GC()

	runtime.ReadMemStats(&baseline)

	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  0,
		FileNamePattern: "",
		Compress:        true,
	}

	rLog := WithComponent("test")

	rotator := NewFileWithRotation(cfg)
	assert.NoError(t, rotator.compress(logFile, rLog))

	require.NoError(t, err)
	runtime.ReadMemStats(&after)

	// THEN totalAlloc doesn't exceed 1mb
	totalAlloc := float64(after.TotalAlloc-baseline.TotalAlloc) / float64(mb300)

	assert.Less(t, totalAlloc, float64(mb300/1024))
}

func TestPurgeFiles(t *testing.T) {
	tmp, err := ioutil.TempDir(os.TempDir(), "newrelic-infra")

	require.NoError(t, err)

	defer func() {
		assert.NoError(t, os.RemoveAll(tmp))
	}()

	logFile := filepath.Join(tmp, "newrelic-infra.log")

	// GIVEN a log files and 5 rotated files
	file, err := disk.OpenFile(logFile, os.O_RDWR|os.O_CREATE, filePerm)
	assert.NoError(t, err)
	assert.NoError(t, file.Close())

	defer func() {
		assert.NoError(t, os.Remove(logFile))
	}()

	rotatedFiles := []string{
		fmt.Sprintf("%s.%d.bk.gz", logFile, 1),
		fmt.Sprintf("%s.%d.bk", logFile, 2),
		fmt.Sprintf("%s.%d.bk", logFile, 3),
		fmt.Sprintf("%s.%d.bk", logFile, 4),
		fmt.Sprintf("%s.%d.bk", logFile, 5),
	}

	// Create dummy files
	for _, rotatedFile := range rotatedFiles {
		rotatedFile, err := disk.OpenFile(rotatedFile, os.O_RDWR|os.O_CREATE, filePerm)

		assert.NoError(t, err)
		assert.NoError(t, rotatedFile.Close())

		time.Sleep(100 * time.Millisecond)
	}

	// Cleanup dummy files at the end.
	defer func() {
		for _, rotatedFile := range rotatedFiles {
			// Ignore this error as some files should be already removed.
			_ = os.Remove(rotatedFile)
		}
	}()

	// WITH a MaxFiles config of 3
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  0,
		FileNamePattern: "newrelic-infra.log.hh.bk",
		Compress:        false,
		MaxFiles:        3,
	}

	rotator := NewFileWithRotation(cfg)

	// Mock the date for filename rename
	rotator.getTimeFn = func() time.Time {
		return time.Date(2022, time.January, 1, 10, 23, 45, 0, time.Local)
	}

	rLog := WithComponent("test")

	// WHEN purgeFiles
	err = rotator.purgeFiles(rLog)
	assert.NoError(t, err)

	// THEN only expected files remain in the directory.
	files, err := ioutil.ReadDir(filepath.Dir(cfg.File))
	assert.NoError(t, err)

	// cfg.MaxFiles plus the current file. We only purge rotated files.
	assert.Len(t, files, cfg.MaxFiles+1)

	assert.Equal(t, files[0].Name(), filepath.Base(logFile))
	assert.Equal(t, files[1].Name(), filepath.Base(rotatedFiles[2]))
	assert.Equal(t, files[2].Name(), filepath.Base(rotatedFiles[3]))
	assert.Equal(t, files[3].Name(), filepath.Base(rotatedFiles[4]))
}

func TestShouldNotPurgeFiles(t *testing.T) {
	tmp, err := ioutil.TempDir(os.TempDir(), "newrelic-infra")

	require.NoError(t, err)

	defer func() {
		assert.NoError(t, os.RemoveAll(tmp))
	}()

	logFile := filepath.Join(tmp, "newrelic-infra.log")

	// GIVEN a log files and 5 rotated files
	file, err := disk.OpenFile(logFile, os.O_RDWR|os.O_CREATE, filePerm)
	assert.NoError(t, err)
	assert.NoError(t, file.Close())

	rotatedFile := fmt.Sprintf("%s.%d", logFile, 1)
	rotated, err := disk.OpenFile(rotatedFile, os.O_RDWR|os.O_CREATE, filePerm)
	require.NoError(t, err)
	require.NoError(t, rotated.Close())

	// WITH a MaxFiles config of 1
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  0,
		FileNamePattern: "",
		Compress:        false,
		MaxFiles:        1,
	}

	rotator := NewFileWithRotation(cfg)

	rLog := WithComponent("test")

	// WHEN purgeFiles
	err = rotator.purgeFiles(rLog)
	assert.NoError(t, err)

	// THEN no files are removed
	files, err := ioutil.ReadDir(filepath.Dir(cfg.File))
	assert.NoError(t, err)

	// cfg.MaxFiles plus the current file. We only purge rotated files.
	assert.Len(t, files, cfg.MaxFiles+1)

	assert.Equal(t, files[0].Name(), filepath.Base(logFile))
	assert.Equal(t, files[1].Name(), filepath.Base(rotatedFile))
}

// TestWithLogger will set a FileWithRotation to the logger and trigger rotation functionality.
// If global logger will be used inside FileWithRotation it can lead to a deadlock.
func TestWithLogger(t *testing.T) {
	tmp, err := ioutil.TempDir(os.TempDir(), "newrelic-infra")
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, os.RemoveAll(tmp))
	}()

	logFile := filepath.Join(tmp, "newrelic-infra.log")

	// GIVEN a new NewFileWithRotation
	cfg := FileWithRotationConfig{
		File:            logFile,
		MaxSizeInBytes:  1000,
		MaxFiles:        1,
		FileNamePattern: "",
		Compress:        true,
	}

	file, err := NewFileWithRotation(cfg).Open()
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, file.Close())
	}()

	// Set file to global logger to make sure there are no deadlocks.
	outBk := w.l.Out
	levelBk := w.l.Level
	SetOutput(file)
	SetLevel(logrus.TraceLevel)

	// restore
	defer func() {
		SetOutput(outBk)
		SetLevel(levelBk)
	}()

	go func() {
		// Write to logger
		for i := 0; i < 100; i++ {
			w.l.Debug("test")
		}
	}()

	// cfg.MaxFiles + current log file
	expectedFiles := cfg.MaxFiles + 1

	var actualFiles int

	// purging and compressing is asynchronous we have to retry.
	require.Eventuallyf(t,
		func() bool {
			files, err := ioutil.ReadDir(tmp)
			assert.NoError(t, err)

			actualFiles = len(files)

			return expectedFiles == actualFiles
		},
		10*time.Second, 100*time.Millisecond,
		"Expected %d files, but got: %d", expectedFiles, actualFiles,
	)
}
