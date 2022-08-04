// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin || linux || freebsd
// +build darwin linux freebsd

package plugins

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

type FileData struct {
	Name     string `json:"id"`
	Size     string `json:"file_size"`
	Mode     string `json:"mode"`
	UID      string `json:"owner_user"`
	GID      string `json:"owner_group"`
	HashMd5  string `json:"md5_hash"`
	FileType string `json:"file_type"`
}

func (self FileData) SortKey() string {
	return self.Name
}

func getFileData(filename string) (d FileData, err error) {
	var stat os.FileInfo
	stat, err = os.Lstat(filename)
	if err != nil {
		return
	}
	d.Name = filename
	d.Size = strconv.FormatInt(stat.Size(), 10)
	d.Mode = stat.Mode().String()
	d.UID = strconv.FormatUint(uint64(stat.Sys().(*syscall.Stat_t).Uid), 10)
	d.GID = strconv.FormatUint(uint64(stat.Sys().(*syscall.Stat_t).Gid), 10)
	d.FileType = fileTypeString(stat)

	var hash []byte
	if stat.Mode().IsRegular() {
		hash, err = helpers.FileMD5(filename)
		if err != nil {
			slog.WithError(err).WithField("file", filename).Error("Could not compute hash for file")
			d.HashMd5 = "unknown"
			err = nil
		} else {
			d.HashMd5 = fmt.Sprintf("%x", hash)
		}
	}
	return
}
