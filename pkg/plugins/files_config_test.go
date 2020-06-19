// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package plugins

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	. "gopkg.in/check.v1"
)

type FilesConfigSuite struct{}

var _ = Suite(&FilesConfigSuite{})

func (s *FilesConfigSuite) TestExternalDFileParsing(c *C) {
	fmt.Println(os.Getwd())
	files, err := parseExternalDFile("fixtures/files_config/external.d/valid01.json")
	c.Assert(err, IsNil)
	c.Assert(len(files), Equals, 2)
	filesByName := make(map[string]bool, 0)
	for _, f := range files {
		filesByName[f] = true
	}
	c.Check(filesByName["/etc/nginx/ssl.conf"], Equals, true)
	c.Check(filesByName["/etc/opsmatic/square_cash_creds"], Equals, true)
}

func (s *FilesConfigSuite) TestExternalDParsing(c *C) {
	files, err := parseExternalD("fixtures/files_config/external.d")
	c.Assert(err, IsNil)
	c.Assert(len(files), Equals, 4)
}

func (s *FilesConfigSuite) TestGetPluginDataset(c *C) {
	var err error

	monitoredFiles, err = parseExternalD("fixtures/files_config/external.d/existing.json")
	c.Assert(err, IsNil)
	dataset, err := getPluginDataset()
	c.Assert(err, IsNil)
	c.Assert(len(dataset), Equals, 1)
	log.Info(dataset)
	c.Check(strings.Contains(dataset[0].(FileData).Name, "/etc/fstab"), Equals, true)
}

func (s *FilesConfigSuite) TestGetFileData(c *C) {
	buf := []byte("foobarbaz\n")
	tmp, err := ioutil.TempFile("", "filedatatest")
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(tmp.Name(), buf, 0644)
	c.Assert(err, IsNil)
	fileData, err := getFileData(tmp.Name())
	c.Assert(err, IsNil)
	c.Check(fileData.HashMd5, Equals, "a0a6e1a375117c58d77221f10c5ce12e")
	c.Check(fileData.Name, Equals, tmp.Name())
	c.Check(fileData.Mode, Equals, "-rw-------")
	c.Check(fileData.UID, Equals, strconv.FormatUint(uint64(os.Getuid()), 10))
	c.Check(fileData.GID, Equals, strconv.FormatUint(uint64(os.Getgid()), 10))
	c.Check(fileData.Size, Equals, strconv.FormatInt(int64(len(buf)), 10))
	c.Check(fileData.FileType, Equals, "regular file")
}

func (s *FilesConfigSuite) TestGetFileDataNonRegFile(c *C) {
	fileData, err := getFileData("/dev/null")
	c.Assert(err, IsNil)
	c.Check(fileData.HashMd5, Equals, "")
	c.Check(fileData.Name, Equals, "/dev/null")
	c.Check(fileData.Mode, Equals, "Dcrw-rw-rw-")
	c.Check(fileData.UID, Equals, "0")
	c.Check(fileData.GID, Equals, "0")
	c.Check(fileData.Size, Equals, "0")
	c.Check(fileData.FileType, Equals, "device")
}

func (s *FilesConfigSuite) TestShouldBeIgnored(c *C) {
	tests := []struct {
		p        string
		expected bool
	}{
		{"/var/log/cassandra.log", true},
		{"/var/log/cassandra/system.log", true},
		{"/var/log/syslog", true},
		{"/var/log/btmp", true},
		{"/var/log/messages", true},
		{"/etc/passwd", false},
		{"/etc/ssl.conf", false},
		{"/etc/shmotd", false},
		{"/etc/motd", true},
	}

	for _, t := range tests {
		c.Check(fmt.Sprintf("%s %t", t.p, isLogFile(t.p)), Equals, fmt.Sprintf("%s %t", t.p, t.expected))
	}
}

func (s *FilesConfigSuite) TestIgnoreNonexistentFiles(c *C) {
	c.Check(shouldBeIgnored("/this/file/is/assumed/to/not/exist"), Equals, true)
}

func (s *FilesConfigSuite) TestIgnoreDirectories(c *C) {
	tmpDir, err := ioutil.TempDir("", "TestIgnoreDirectories")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "TestFile")
	err = ioutil.WriteFile(path, []byte("test"), 0644)
	if err != nil {
		c.Fatal(err)
	}

	c.Check(shouldBeIgnored(tmpDir), Equals, true)
	c.Check(shouldBeIgnored(path), Equals, false)
}
