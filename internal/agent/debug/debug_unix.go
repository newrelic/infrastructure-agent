// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package debug

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/pkg/errors"
)

func init() {
	ProvideFn = func() (string, error) {
		fdCount, err := fileDescriptorCount()

		return fmt.Sprintf("resource usage report: bytes allocated: %d file descriptors: %d", memAllocBytes(), fdCount), err
	}
}

func fileDescriptorCount() (fdCount int, err error) {
	fdCount = -1

	fds, err := readFDs()
	if err != nil {
		err = errors.Wrap(err, "unable to determine open descriptor count for agent")
	} else {
		fdCount = len(fds)
	}

	return
}

// readFDs reads the file descriptors for the current Linux process
func readFDs() ([]os.FileInfo, error) {
	return ioutil.ReadDir(helpers.HostProc(strconv.Itoa(os.Getpid()), "fd"))
}
