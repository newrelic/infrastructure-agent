// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package log

import (
	"compress/gzip"
	"fmt"
	"io"
)

const (
	// compressedFileExt is the extension used for the compressed rotated file.
	compressedFileExt = "gz"
)

// compressContent will compress the provided content.
func copyContentToArchive(_ string, src io.Reader, dst io.Writer, log Entry) error {
	gzFile := gzip.NewWriter(dst)

	defer func() {
		if err := gzFile.Close(); err != nil {
			log.Debug("Failed to close gzip writer after rotating the log file")
		}
	}()

	_, err := io.Copy(gzFile, src)
	if err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}

	return nil
}
