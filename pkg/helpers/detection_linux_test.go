// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package helpers

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"
)

type DetectionSuite struct{}

var _ = Suite(&DetectionSuite{})
var osRelease string

var (
	CENTOS = []byte(`
NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:centos:centos:7"
HOME_URL="https://www.centos.org/"
BUG_REPORT_URL="https://bugs.centos.org/"

CENTOS_MANTISBT_PROJECT="CentOS-7"
CENTOS_MANTISBT_PROJECT_VERSION="7"
REDHAT_SUPPORT_PRODUCT="centos"
REDHAT_SUPPORT_PRODUCT_VERSION="7"`,
	)

	COREOS = []byte(`
NAME="CoreOS"
ID=coreos
VERSION=835.9.0
VERSION_ID=835.9.0
BUILD_ID=
PRETTY_NAME="CoreOS 835.9.0"
ANSI_COLOR="1;32"
HOME_URL="https://coreos.com/"
BUG_REPORT_URL="https://github.com/coreos/bugs/issues"`,
	)
)

func (s *DetectionSuite) TestGetLinuxDistroCoreOS(c *C) {
	tmpEtc, err := ioutil.TempDir("", "/testing")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpEtc)

	tmpEtc2 := filepath.Join(tmpEtc, "os-release")
	if err := ioutil.WriteFile(tmpEtc2, COREOS, 0666); err != nil {
		log.Fatal(err)
	}
	os.Setenv("HOST_ETC", tmpEtc)
	val := GetLinuxDistro()
	c.Assert(val, Equals, LINUX_COREOS)
}

func (s *DetectionSuite) TestGetLinuxDistro(c *C) {
	tmpEtc, err := ioutil.TempDir("", "/testing")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpEtc)

	tmpEtc2 := filepath.Join(tmpEtc, "os-release")
	if err := ioutil.WriteFile(tmpEtc2, CENTOS, 0666); err != nil {
		log.Fatal(err)
	}
	os.Setenv("HOST_ETC", tmpEtc)
	val := GetLinuxDistro()
	c.Assert(val, Equals, LINUX_REDHAT)
}

func (s *DetectionSuite) TestGetLinuxOSInfo(c *C) {
	tmpEtc, err := ioutil.TempDir("", "/testing")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpEtc)

	tmpEtc2 := filepath.Join(tmpEtc, "os-release")
	if err := ioutil.WriteFile(tmpEtc2, CENTOS, 0666); err != nil {
		log.Fatal(err)
	}
	os.Setenv("HOST_ETC", tmpEtc)

	val, terr := GetLinuxOSInfo()

	c.Assert(terr, IsNil)
	c.Assert(val, HasLen, 14)
	c.Assert(val["PRETTY_NAME"], Equals, "CentOS Linux 7 (Core)")
	c.Assert(val["ID_NOTHING"], Equals, "")
}

func (s *DetectionSuite) TestIsAmazonOS(c *C) {
	tmpEtc, err := ioutil.TempDir("", "/testing")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpEtc)

	tmpEc2 := filepath.Join(tmpEtc, "ec2_version")
	if err := ioutil.WriteFile(tmpEc2, []byte("junk"), 0666); err != nil {
		log.Fatal(err)
	}
	os.Setenv("HOST_ETC", filepath.Dir(tmpEc2))
	c.Assert(IsAmazonOS(), Equals, true)
}
