// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"archive/zip"
	"fmt"
	"io"
)

const (
	// compressedFileExt is the extension used for the compressed rotated file.
	compressedFileExt = "zip"
)

// compressContent will compress the provided content.
func copyContentToArchive(srcFileName string, src io.Reader, dst io.Writer, log Entry) error {
	zipFile := zip.NewWriter(dst)

	defer func() {
		if err := zipFile.Close(); err != nil {
			log.Debug("Failed to close zip writer after rotating the log file")
		}
	}()

	zipWriter, err := zipFile.Create(srcFileName)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	_, err = io.Copy(zipWriter, src)
	if err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}

	return nil
}
