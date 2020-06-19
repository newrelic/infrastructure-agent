// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestSysvInitPluginRun tests that the plugin finds the mync process. In
// order for the process to be discovered it needs to have a pid file in
// /var/run/ and a Ppid equal to one. To get the Ppid = 1 we run a script
// that executes the command in a subshell, which assigns the init
// process (pid = 1) as parent to our process.
func TestSysvInitPluginRun(t *testing.T) {
	t.Skip("untestable plugin, refactor required")

	if os.Getuid() != 0 {
		t.Skip("Test can only be run as root")
	}

	content := []byte(`#!/bin/bash
(nc -l -p 12345  &)
pgrep -f 'nc -l -p 12345' > /var/run/mync.pid`)
	tmpfile, err := ioutil.TempFile("", "mync.sh")
	if err != nil {
		logrus.Error(err)
	}

	defer func() {
		// Kill process
		pidFile := "/var/run/mync.pid"
		pidBytes, err := ioutil.ReadFile(pidFile)
		if err != nil {
			logrus.Error(err)
		}
		pid := fmt.Sprintf("%s", bytes.TrimSpace(pidBytes))
		err = exec.Command("kill", pid).Run()
		if err != nil {
			logrus.Error(err)
		}
		// Remove files
		err = os.Remove(pidFile)
		if err != nil {
			logrus.Error(err)
		}
		err = os.Remove(tmpfile.Name())
		if err != nil {
			logrus.Error(err)
		}
	}()

	// Setup process with Ppid = 1
	if _, err := tmpfile.Write(content); err != nil {
		logrus.Error(err)
	}
	if err := tmpfile.Close(); err != nil {
		logrus.Error(err)
	}
	command := exec.Command("/bin/bash", tmpfile.Name())
	if err := command.Run(); err != nil {
		logrus.Error(err)
	}

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	pluginId := ids.PluginID{"services", "pidfile"}
	sysvInitPlugin := pluginsLinux.NewSysvInitPlugin(pluginId, a.Context)
	a.RegisterPlugin(sysvInitPlugin)

	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(testhelpers.InventoryDuration(a.Context.Config().SysvInitIntervalSec)):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// Check that it founds our mync process
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "services/pidfile",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"mync": map[string]interface{}{
					"gids":         "0 0 0 0",
					"id":           "mync",
					"listen_socks": "[\"TCP:0.0.0.0:12345\"]",
					"pid":          fixture.AnyValue,
					"ppid":         "1",
					"uids":         "0 0 0 0",
				},
			},
		},
	})
}
