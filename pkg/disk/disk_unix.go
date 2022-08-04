// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin || freebsd
// +build linux darwin freebsd

// package disk provides access to common disk write operations
package disk

import (
	"io/ioutil"
	"os"
)

// WriteFile is a façade to ioutil.WriteFile, which enforces safe disk access when the host configuration requires it
var WriteFile = ioutil.WriteFile

// OpenFile is a façade to os.OpenFile, which enforces safe disk access when the host configuration requires it
var OpenFile = os.OpenFile

// Create is a façade to os.Create, which enforces safe disk access when the host configuration requires it
var Create = os.Create

// MkdirAll is a façade to os.MkdirAll, which enforces safe disk access when the host configuration requires it
var MkdirAll = os.MkdirAll
