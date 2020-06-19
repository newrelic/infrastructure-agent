// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// package disk provides safe access to common windows disk write operations
package disk

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
)

// WriteFile writes data to a file named by filename. For more info, see ioutil.Writefile.
// If the file does not exist, WriteFile creates it with permissions perm;
// otherwise WriteFile truncates it before writing.
// If it is not safe to write to the file, the operation is aborted and an error is returned.
// It is not safe to write into a file when in windows we are trying to write into a symbolic link.
func WriteFile(filePath string, data []byte, perm os.FileMode) error {
	err := checkWriteSafe(filePath)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, perm)
}

// OpenFile is the generalized open call. For more info, see os.OpenFile
// If the file is open for writing or appending and it is not safe to write the file, the operation is aborted and an
// error is returned. It is not safe to write into a file when in windows we are trying to write into a symbolic link.
// If there is another error, it will be of type *PathError.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	// Do not verify Read-only files
	if flag&os.O_RDONLY == 0 {
		if err := checkWriteSafe(name); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(name, flag, perm)
}

// Create creates the named file with mode 0666 (before umask), truncating
// it if it already exists. For more info, see os.Create.
// If it is not safe to write to the file, the operation is aborted and an error is returned.
// It is not safe to write into a file when in windows we are trying to write into a symbolic link.
func Create(fileName string) (*os.File, error) {
	if err := checkWriteSafe(fileName); err != nil {
		return nil, err
	}
	return os.Create(fileName)
}

// MkdirAll creates a directory path along with any necessary parents. For more info, see os.MkdirAll.
// If the directory is not considered safe, the operation is aborted and an error is returned. It is not safe if any
// element of the path that already exists is a Windows junction folder
func MkdirAll(path string, perm os.FileMode) error {
	if err := checkSafeDir(path); err != nil {
		return err
	}
	return os.MkdirAll(path, perm)
}

// checkWriteSafe checks whether a file is safe to access for write in Windows. This is: it is not safe to access if the
// file has hard links (or is a hard link)
func checkWriteSafe(filePath string) error {
	if err := checkSafeDir(filepath.Dir(filePath)); err != nil {
		return err
	}
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	defer file.Close()

	if err != nil {
		// If file does not exist, it can be safely created
		if _, ok := err.(*os.PathError); ok {
			return nil
		}
		return err
	}
	return checkWriteSafeF(file)
}

// checkWriteSafeF provides the base functionality for checkWriteSafe, but receiving a file handler
func checkWriteSafeF(file *os.File) error {
	fileInfo := syscall.ByHandleFileInformation{}
	err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &fileInfo)
	if err != nil {
		return err
	}

	if fileInfo.NumberOfLinks > 1 {
		return fmt.Errorf("writing to hard links is not allowed: %s", file.Name())
	}
	return nil
}

// checkSafeDir checks the safety of a write directory. This is: it does not have any ancestor that is a junction (or it
// is a junction itself)
func checkSafeDir(dirPath string) error {
	// Checking if the directory is a junction, or it has any ancestor as a junction
	folder, err := filepath.Abs(dirPath)
	if err != nil {
		return err
	}
	child := ""
	// Ignoring trailing non-existent directories
	for folder != child { // While we don't reach the drive letter
		if _, err = os.Stat(folder); err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		child = folder
		folder = filepath.Dir(folder)
	}

	for folder != child { // While we don't reach the drive letter
		stat, err := os.Lstat(folder)
		if err != nil {
			return err
		}
		if stat.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("junctions and symlinks are not allowed: %s", folder)
		}
		child = folder
		folder = filepath.Dir(folder)
	}

	return nil
}
