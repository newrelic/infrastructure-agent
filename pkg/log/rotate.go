// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/disk"
)

const (
	// defaultDatePattern used to generate filename for the rotated file.
	defaultDatePattern = "YYYY-MM-DD_hh-mm-ss"
	// filePerm specified the permissions while opening a file.
	filePerm = 0o600
)

// ErrFileNotOpened is returned when an operation cannot be performed because the file is not opened.
var ErrFileNotOpened = errors.New("cannot perform operation, file is not opened")

// FileWithRotationConfig keeps the configuration for a new FileWithRotation.
type FileWithRotationConfig struct {
	File            string
	FileNamePattern string
	MaxSizeInBytes  int64
	Compress        bool
	MaxFiles        int
}

// FileWithRotation decorates a file with rotation mechanism.
// The current file will be rotated before Write(ing) new content if that will cause exceeding the
// configured max bytes. The rotated file will get the name from the provided pattern in the configuration.
// If rotation fails, we will continue to write to the current log file to avoid losing data.
//
// Global logger should not be called within the synchronous methods of FileWithRotation since it can
// lead to a deadlock. Global logger can be called from Asynchronous code.
type FileWithRotation struct {
	sync.Mutex
	cfg FileWithRotationConfig

	file *os.File

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
func (f *FileWithRotation) Write(content []byte) (int, error) {
	f.Lock()
	defer f.Unlock()

	newContentSize := int64(len(content))

	// Make sure new content fits the max size from the configuration.
	if newContentSize > f.cfg.MaxSizeInBytes {
		return 0, fmt.Errorf("failed to write to file, new content size: '%db' exceeds to maximum file size: '%db'",
			newContentSize, f.cfg.MaxSizeInBytes)
	}

	// Check if the file should be rotated.
	if f.cfg.MaxSizeInBytes > 0 && f.writtenBytes+newContentSize > f.cfg.MaxSizeInBytes {
		// Generate the rotation filename according to the config.
		dir := filepath.Dir(f.cfg.File)
		newFile := filepath.Join(dir, f.generateFileName())

		err := f.rotate(newFile)
		// If rotation fails, we should try to continue logging in the same file.
		if err != nil {
			if openErr := f.open(); openErr != nil {
				return 0, fmt.Errorf("failed to re-open file after rotate failed, error: %w", openErr)
			}
		} else {
			f.asyncPostRotateActions(newFile)
		}
	}

	if f.file == nil {
		return 0, ErrFileNotOpened
	}

	writtenBytes, err := f.file.Write(content)
	f.writtenBytes += int64(writtenBytes)

	return writtenBytes, err
}

func (f *FileWithRotation) open() error {
	var err error

	f.file, err = disk.OpenFile(f.cfg.File, os.O_RDWR|os.O_CREATE|os.O_APPEND, filePerm)
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
func (f *FileWithRotation) rotate(newFile string) error {
	if f.file == nil {
		return ErrFileNotOpened
	}

	if err := f.file.Close(); err != nil {
		return fmt.Errorf("failed to rotate file, error while closing the current file: %v", err)
	}

	if err := os.Rename(f.cfg.File, newFile); err != nil {
		return fmt.Errorf("failed to rotate file, error while moving the current file: %v", err)
	}

	if err := f.open(); err != nil {
		return fmt.Errorf("failed to create new file after rotation, error: %v", err)
	}

	return nil
}

func (f *FileWithRotation) asyncPostRotateActions(rotatedFile string) {
	go func() {
		rLog := WithComponent("LogRotator")

		rLog.Debugf("File %s rotated to: %s", f.cfg.File, rotatedFile)

		// Clean old files if MaxFiles is exceeded.
		if err := f.purgeFiles(rLog); err != nil {
			rLog.WithError(err).Error("Failed to clean old rotated log files")
		}

		if f.cfg.Compress {
			if err := f.compress(rotatedFile, rLog); err != nil {
				rLog.WithError(err).Error("Failed to compress rotated log file")

				return
			}
			// Clean file that was compressed.
			if err := os.Remove(rotatedFile); err != nil {
				rLog.WithError(err).Error("Failed to clean rotated log file after was compressed")

				return
			}
		}
	}()
}

// compress will create a .gz archive for the file provided.
func (f *FileWithRotation) compress(file string, log Entry) error {
	dst := fmt.Sprintf("%s.%s", file, compressedFileExt)

	log.Debugf("Compressing log file: %s to: %s", file, dst)

	srcFile, err := disk.OpenFile(file, os.O_RDWR|os.O_CREATE, filePerm)
	if err != nil {
		return fmt.Errorf("failed to compress rotated file: %s, error: %w", file, err)
	}

	defer func() {
		if err = srcFile.Close(); err != nil {
			log.Debugf("Failed to close original file: %s after being rotated", file)
		}
	}()

	srcReader := bufio.NewReader(srcFile)

	dstFile, err := disk.OpenFile(dst, os.O_RDWR|os.O_CREATE, filePerm)
	if err != nil {
		return fmt.Errorf("failed to compress rotated file: %s, error: %w", file, err)
	}

	defer func() {
		if err = dstFile.Close(); err != nil {
			log.Debugf("Failed to close destination file: %s after original was rotated", dst)
		}
	}()

	dstWriter := bufio.NewWriter(dstFile)

	defer func() {
		if err = dstWriter.Flush(); err != nil {
			log.Debugf("Failed to flush remaining buffer data while rotating to file: %s", dst)
		}
	}()

	err = copyContentToArchive(filepath.Base(file), srcReader, dstWriter, log)
	if err != nil {
		return fmt.Errorf("failed to compress rotated file: %s, error: %w", file, err)
	}
	return nil
}

// purgeFiles will remove older files in case MaxFiles is exceeded.
func (f *FileWithRotation) purgeFiles(log Entry) error {
	if f.cfg.MaxFiles < 1 {
		// Nothing to do.
		return nil
	}

	dir := filepath.Dir(f.cfg.File)

	globPattern := f.generateFileNameGlob()
	// Get only files that match the pattern. Add star at the end to match also compressed files.
	matches, err := filepath.Glob(filepath.Join(dir, globPattern+"*"))
	if err != nil {
		return fmt.Errorf("could not retrieve files matching the pattern: %s, error: %w", globPattern, err)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to purge old rotated files, error: %w", err)
	}

	filteredFiles := make([]fs.FileInfo, 0)

	for _, file := range files {
		// Keep only files that match the pattern.
		for _, match := range matches {
			if filepath.Join(dir, file.Name()) == match {
				filteredFiles = append(filteredFiles, file)
			}
		}
	}

	if len(filteredFiles) <= f.cfg.MaxFiles {
		// Nothing to do.
		return nil
	}

	// Sort files by last modification time, the newest first.
	sort.Slice(filteredFiles, func(i, j int) bool {
		return filteredFiles[i].ModTime().After(filteredFiles[j].ModTime())
	})

	// Remove older files.
	for _, file := range filteredFiles[f.cfg.MaxFiles:] {
		fileName := filepath.Join(dir, file.Name())

		log.Debugf("Purging old file: %s", fileName)

		if err := os.Remove(fileName); err != nil {
			return fmt.Errorf("failed to purge old rotated files, error: %w", err)
		}
	}

	return nil
}

// generateFileName will use the specified pattern to create a new filename when the current file is rotated.
// If the pattern is not specified in the configuration, by default a new filename will be created with
// the following pattern: current_file_name_defaultDatePattern.current_file_extension.
func (f *FileWithRotation) generateFileName() string {
	pattern := f.getFileNamePattern()

	return formatTime(pattern, f.getTimeFn())
}

// generateFileNameGlob will generate a glob for the pattern to match only files with the same
// pattern while cleaning old files.
func (f *FileWithRotation) generateFileNameGlob() string {
	pattern := f.getFileNamePattern()

	for token := range getTokenReplacers(time.Time{}) {
		pattern = strings.ReplaceAll(pattern, token, "*")
	}

	return pattern
}

// getFileNamePattern will provide the configured filename pattern for the rotated file.
// If a custom pattern for the rotated filename wasn't provided, we generated one based on the default values.
func (f *FileWithRotation) getFileNamePattern() string {
	if f.cfg.FileNamePattern != "" {
		return f.cfg.FileNamePattern
	}

	// Insert time into the log filename.
	ext := filepath.Ext(f.cfg.File)
	fileName := filepath.Base(f.cfg.File)
	fileName = strings.TrimSuffix(fileName, ext)

	return fmt.Sprintf("%s_%s%s", fileName, defaultDatePattern, ext)
}

// formatTime will receive a time object and a pattern to format the current time.
func formatTime(pattern string, ts time.Time) string {
	for token, replacer := range getTokenReplacers(ts) {
		pattern = strings.ReplaceAll(pattern, token, replacer)
	}

	return pattern
}

// getTokenReplacers returns a map of the supported timestamp tokens with the replacer value.
func getTokenReplacers(ts time.Time) map[string]string {
	return map[string]string{
		"YYYY": fmt.Sprintf("%d", ts.Year()),
		"MM":   fmt.Sprintf("%02d", ts.Month()),
		"DD":   fmt.Sprintf("%02d", ts.Day()),
		"hh":   fmt.Sprintf("%02d", ts.Hour()),
		"mm":   fmt.Sprintf("%02d", ts.Minute()),
		"ss":   fmt.Sprintf("%02d", ts.Second()),
	}
}
