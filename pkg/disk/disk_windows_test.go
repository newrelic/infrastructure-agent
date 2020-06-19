// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package disk

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// mkLink makes an NTFS junction
func mkLink(link, target string) error {
	// os.Symlink does not properly work for windows folders
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if err := cmd.Run(); err != nil {
		out, _ := cmd.CombinedOutput()
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

// creates a path of folders on top of a base folder and returns the full, joined path
// If the folder name starts with *, a directory junction will be created, named as the rest of the string
// If the folder name starts with !, no more folders will be created but those will be joined in the result
func createPath(base string, folders ...string) (string, error) {
	linkTarget := ""
	path := base
	i := 0
	for ; i < len(folders) && folders[i][0] != '!'; i++ {

		if folders[i][0] == '*' { // creating folder junction
			path = filepath.Join(path, folders[i][1:])
			// Create some folder to link to
			if linkTarget == "" {
				linkTarget = filepath.Join(base, "link-target")
				if err := os.Mkdir(linkTarget, os.ModeDir); err != nil && !os.IsExist(err) {
					return "", err
				}
			}
			mkLink(path, linkTarget)
		} else { // creating normal folder
			path = filepath.Join(path, folders[i])
			if err := os.Mkdir(path, os.ModeDir); err != nil && !os.IsExist(err) {
				return "", err
			}
		}
	}
	// appending strings to path without creating folders
	for ; i < len(folders); i++ {
		if folders[i][0] == '!' {
			path = filepath.Join(path, folders[i][1:])
		} else {
			path = filepath.Join(path, folders[i])
		}
	}
	return path, nil
}

func TestWriteFile_Allowed(t *testing.T) {
	// Given a file that does not exist
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	outFile := path.Join(dir, "my-file")

	// When the write operation is performed
	err = WriteFile(outFile, []byte("Hello World"), 0644)

	// The system does not return error
	require.NoError(t, err)

	// And the new file is accessible
	b, err := ioutil.ReadFile(outFile)
	require.NoError(t, err)
	require.Equal(t, "Hello World", string(b))
}

func TestWriteFile_Allowed_Truncate(t *testing.T) {
	// Given an existing file
	outFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	err = WriteFile(outFile.Name(), []byte("this will be erased"), 0644)
	require.NoError(t, outFile.Close())

	// When the write operation is performed
	err = WriteFile(outFile.Name(), []byte("Hello World"), 0644)

	// The system does not return error
	require.NoError(t, err)

	// And the new file content is accessible
	b, err := ioutil.ReadFile(outFile.Name())
	require.NoError(t, err)
	require.Equal(t, "Hello World", string(b))
}

func TestWriteFile_Disallowed(t *testing.T) {
	// Given an existing file
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	outFile := path.Join(dir, "my-file")
	err = WriteFile(outFile, []byte("this is not going to be erased"), 0644)
	require.NoError(t, err)

	// and its symlink
	symLink := path.Join(dir, "symlink")

	// os.Symlink does not properly work on Windows
	require.NoError(t, exec.Command("fsutil", "hardlink", "create", symLink, outFile).Run())

	// When the write operation is performed
	err = WriteFile(outFile, []byte("Hello World"), 0644)

	// The system returns an error
	require.Error(t, err)

	// And the data is not erased
	b, err := ioutil.ReadFile(outFile)
	require.NoError(t, err)
	require.Equal(t, "this is not going to be erased", string(b))

	b, err = ioutil.ReadFile(symLink)
	require.NoError(t, err)
	require.Equal(t, "this is not going to be erased", string(b))
}

func TestOpenFile_Allowed(t *testing.T) {
	// Given a file that does not exist
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	outFile := path.Join(dir, "my-file")

	// When the file is open for writing
	file, err := OpenFile(outFile, os.O_WRONLY|os.O_CREATE, 0666)
	defer file.Close()

	// The system does not return error
	require.NoError(t, err)
	// And a handler is returned
	require.NotNil(t, file)
}

func TestOpenFile_Allowed_Exists(t *testing.T) {
	// Given an existing file
	outFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	err = WriteFile(outFile.Name(), []byte("temp file"), 0644)
	require.NoError(t, outFile.Close())

	// When the file is open for writing
	file, err := OpenFile(outFile.Name(), os.O_WRONLY|os.O_CREATE, 0666)
	defer file.Close()

	// The system does not return error
	require.NoError(t, err)
	// And a handler is returned
	require.NotNil(t, file)
}

func TestOpenFile_Allowed_ReadOnly(t *testing.T) {
	// Given an existing file
	outFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	err = WriteFile(outFile.Name(), []byte("temp file"), 0644)
	require.NoError(t, outFile.Close())

	// When the file is open as read_only
	file, err := OpenFile(outFile.Name(), os.O_RDONLY, 0666)
	defer file.Close()

	// The system does not return error
	require.NoError(t, err)
	// And a handler is returned
	require.NotNil(t, file)
}

func TestOpenFile_Disallowed(t *testing.T) {
	// Given an existing file
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	outFile := path.Join(dir, "my-file")
	err = WriteFile(outFile, []byte("temp file"), 0644)
	require.NoError(t, err)

	// and its symlink
	symLink := path.Join(dir, "symlink")

	// os.Symlink does not properly work on Windows
	require.NoError(t, exec.Command("fsutil", "hardlink", "create", symLink, outFile).Run())

	// When the file is open for writing
	_, err = OpenFile(outFile, os.O_WRONLY|os.O_CREATE, 0666)

	// The system returns an error
	require.Error(t, err)
}

func TestCreate_Allowed(t *testing.T) {
	// Given a file that does not exist
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	outFile := path.Join(dir, "my-file")

	// When the file is open for Creating
	file, err := Create(outFile)
	defer file.Close()

	// The system does not return error
	require.NoError(t, err)
	// And a handler is returned
	require.NotNil(t, file)
}

func TestCreate_Allowed_Exists(t *testing.T) {
	// Given an existing file
	outFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	err = WriteFile(outFile.Name(), []byte("temp file"), 0644)
	require.NoError(t, outFile.Close())

	// When the file is open for Truncating
	file, err := Create(outFile.Name())
	defer file.Close()

	// The system does not return error
	require.NoError(t, err)
	// And a handler is returned
	require.NotNil(t, file)
}

func TestCreate_Disallowed(t *testing.T) {
	// Given an existing file
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	outFile := path.Join(dir, "my-file")
	err = WriteFile(outFile, []byte("temp file"), 0644)
	require.NoError(t, err)

	// and its symlink
	symLink := path.Join(dir, "symlink")

	// os.Symlink does not properly work on Windows
	require.NoError(t, exec.Command("fsutil", "hardlink", "create", symLink, outFile).Run())

	// When the system tries to create/truncate the file
	_, err = Create(outFile)

	// The system returns an error
	require.Error(t, err)
}

func TestCreate_Disallowed_BecauseJunction(t *testing.T) {
	// Given a file to be created for writing into a Junction
	basePath, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	dir, err := createPath(basePath, "*junction")
	require.NoError(t, err)

	outFile := filepath.Join(dir, "someFile.txt")

	// When the system tries to create/truncate the file
	_, err = Create(outFile)

	// The system returns an error
	require.Error(t, err)

	// and the file is not created
	_, err = os.Lstat(outFile)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func TestMkdirAll_Allowed(t *testing.T) {
	// Given a set of directories that should be allowed
	testCases := []struct {
		description      string
		creationFunction func(string) (string, error)
	}{
		{"path without junctions", func(base string) (string, error) {
			return createPath(base, "some", "dir", "structure")
		}},
		{"non-existing folder", func(base string) (string, error) {
			return createPath(base, "existing", "!notExisting")
		}},
		{"non-existing folder with a non-existing parent", func(base string) (string, error) {
			return createPath(base, "existing", "!notExisting", "neitherExisting")
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			basePath, err := ioutil.TempDir("", "")
			require.NoError(t, err)

			// Given a directories path with the characteristics given by the test case
			directory, err := testCase.creationFunction(basePath)
			require.NoError(t, err)

			// The system accepts writing to it
			require.NoError(t, MkdirAll(directory, os.ModeDir))

			// and the directories are created
			require.DirExists(t, directory)
		})
	}
}

func TestMkdir_Disallowed(t *testing.T) {
	// Given a set of directories that should not be allowed
	testCases := []struct {
		description      string
		creationFunction func(string) (string, error)
	}{
		{"junction folder", func(base string) (string, error) {
			return createPath(base, "some", "*junction")
		}},
		{"folder with a junction ancestor", func(base string) (string, error) {
			return createPath(base, "*ancestor", "folder")
		}},
		{"non-existing folder wit a junction parent", func(base string) (string, error) {
			return createPath(base, "*existing", "!notExisting")
		}},
		{"non-existing folder with a junction grandparent", func(base string) (string, error) {
			return createPath(base, "*junction", "existing", "!notExisting", "neitherExisting")
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			basePath, err := ioutil.TempDir("", "")
			require.NoError(t, err)

			// Given a directories path with the characteristics given by the test case
			directory, err := testCase.creationFunction(basePath)
			require.NoError(t, err)

			// The system refuses creating it
			require.Error(t, MkdirAll(directory, os.ModeDir))
		})
	}
}
