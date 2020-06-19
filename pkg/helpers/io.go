// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"fmt"
	"io"
	"os"

	"github.com/newrelic/infrastructure-agent/pkg/disk"
)

func CopyFile(src, dest string) error {
	existingFileData, err := os.Stat(dest)
	if err == nil {
		if existingFileData.IsDir() {
			return fmt.Errorf("cannot copy file %v in place of directory %v.", src, dest)
		}

		// Dest file already exists, delete it first.
		if err = os.Remove(dest); err != nil {
			return err
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	srcFileData, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := disk.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	// Windows doesn't support Chmod but it generally matters less anyway since we do this to preserve
	// execute bits on Linux and there's no permission required to execute a file on Windows.
	if GetOS() != OS_WINDOWS {
		if err = out.Chmod(srcFileData.Mode()); err != nil {
			return err
		}
	}

	return out.Close()
}
