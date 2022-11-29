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

	zipWriter, err := zipFile.Create(srcFileName)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", srcFileName, err)
	}

	_, err = io.Copy(zipWriter, src)
	if err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}

	defer func() {
		if err = zipFile.Close(); err != nil {
			log.Debug("Failed to close zip writer after rotating the log file")
		}
	}()
	return nil
}
