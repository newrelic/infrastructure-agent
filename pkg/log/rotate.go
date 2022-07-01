// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var rLog = WithComponent("LogRotator")

// defaultDatePattern used to generate filename for the rotated file.
const defaultDatePattern = "YYYY-MM-DD_hh-mm-ss"

var (
	// ErrFileNotOpened is returned when an operation cannot be performed because the file is not opened.
	ErrFileNotOpened = errors.New("cannot perform operation, file is not opened")
)

// FileWithRotationConfig keeps the configuration for a new FileWithRotation.
type FileWithRotationConfig struct {
	File            string
	FileNamePattern string
	MaxSizeInBytes  int64
}

// FileWithRotation decorates a file with rotation mechanism.
type FileWithRotation struct {
	sync.Mutex
	cfg FileWithRotationConfig

	file         *os.File
	writtenBytes int64

	getTimeFn func() time.Time
}

// NewFileWithRotation creates a new FileWithRotation.
func NewFileWithRotation(cfg FileWithRotationConfig) *FileWithRotation {
	return &FileWithRotation{
		cfg:       cfg,
		getTimeFn: time.Now,
	}
}

// Open the file to write in. If the file doesn't exist, a new file will be created.
func (f *FileWithRotation) Open() (*FileWithRotation, error) {
	f.Lock()
	defer f.Unlock()

	return f, f.open()
}

// Close the file.
func (f *FileWithRotation) Close() error {
	f.Lock()
	defer f.Unlock()

	if f.file == nil {
		return ErrFileNotOpened
	}

	return f.file.Close()
}

// Write will check if the new content can be written into the file. If not, the file will be
// automatically rotated.
func (f *FileWithRotation) Write(b []byte) (n int, err error) {
	f.Lock()
	defer f.Unlock()

	newContentSize := int64(len(b))

	// Make sure new content fits the max size from the configuration.
	if newContentSize > f.cfg.MaxSizeInBytes {
		return 0, fmt.Errorf("failed to write to file, new content size: '%db' exceeds to maximum file size: '%db'",
			newContentSize, f.cfg.MaxSizeInBytes)
	}

	// Check if the file should be rotated.
	if f.cfg.MaxSizeInBytes > 0 && f.writtenBytes+newContentSize > f.cfg.MaxSizeInBytes {
		err = f.rotate()

		// If rotation fails, we should try to continue logging in the same file.
		if err != nil {
			if openErr := f.open(); openErr != nil {
				return 0, fmt.Errorf("failed to re-open file after rotate failed, error: %v", openErr)
			}

			rLog.WithError(err).Error("Failed to rotate log file")
		}
	}

	if f.file == nil {
		return 0, ErrFileNotOpened
	}

	writtenBytes, err := f.file.Write(b)
	f.writtenBytes += int64(writtenBytes)

	return writtenBytes, err
}

func (f *FileWithRotation) open() error {
	var err error
	f.file, err = disk.OpenFile(f.cfg.File, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open file rotate, error: %v", err)
	}

	if f.file == nil {
		return ErrFileNotOpened
	}

	fileStat, err := f.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to open file rotate, error while reading file stat: %v", err)
	}

	f.writtenBytes = fileStat.Size()
	return nil
}

// rotate will rename the current file according to the filename pattern and will open a new file.
func (f *FileWithRotation) rotate() error {
	if f.file == nil {
		return ErrFileNotOpened
	}

	if err := f.file.Close(); err != nil {
		return fmt.Errorf("failed to rotate file, error while closing the current file: %v", err)
	}

	// Generate the rotation filename according to the config.
	dir := filepath.Dir(f.cfg.File)
	rotateFileName := filepath.Join(dir, f.generateFileName())

	if err := os.Rename(f.cfg.File, rotateFileName); err != nil {
		return fmt.Errorf("failed to rotate file, error while moving the current file: %v", err)
	}

	if err := f.open(); err != nil {
		return fmt.Errorf("failed to create new file after rotation, error: %v", err)
	}

	return nil
}

// generateFileName will use the specified pattern to create a new filename when the current file is rotated.
// If the pattern is not specified in the configuration, by default a new filename will be created with
// the following pattern: current_file_name_defaultDatePattern.current_file_extension.
func (f *FileWithRotation) generateFileName() string {

	pattern := f.cfg.FileNamePattern

	// If a custom pattern for the rotated filename wasn't provided, generated one.
	if pattern == "" {
		// Insert time into the log filename.
		ext := filepath.Ext(f.cfg.File)
		fileName := filepath.Base(f.cfg.File)
		fileName = strings.TrimSuffix(fileName, ext)

		pattern = fmt.Sprintf("%s_%s%s", fileName, defaultDatePattern, ext)
	}

	return formatTime(pattern, f.getTimeFn())
}

// formatTime will receive a time object and a pattern to format the current time.
func formatTime(pattern string, ts time.Time) string {
	tokens := map[string]string{
		"YYYY": fmt.Sprintf("%d", ts.Year()),
		"MM":   fmt.Sprintf("%02d", ts.Month()),
		"DD":   fmt.Sprintf("%02d", ts.Day()),
		"hh":   fmt.Sprintf("%02d", ts.Hour()),
		"mm":   fmt.Sprintf("%02d", ts.Minute()),
		"ss":   fmt.Sprintf("%02d", ts.Second()),
	}

	for token, replacer := range tokens {
		pattern = strings.Replace(pattern, token, replacer, -1)
	}
	return pattern
}
