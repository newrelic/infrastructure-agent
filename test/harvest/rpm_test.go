// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux && harvest
// +build linux,harvest

package harvest

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"

	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestRpmPlugin(t *testing.T) {
	if _, err := os.Stat(pluginsLinux.RpmPath); os.IsNotExist(err) {
		t.Skip("This test must be executed in RPM-based distributions")
	}
	rpmDistro, err := rpmBasedDistribution()
	if err != nil {
		assert.FailNow(t, fmt.Sprintf("cannot detect if rpmBasedDistribution %v", err))
	}
	if !rpmDistro {
		t.Skip()
	}

	testhelpers.SetupLog()

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})
	a.RegisterPlugin(pluginsLinux.NewRpmPlugin(a.Context))
	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(testhelpers.InventoryDuration(a.Context.Config().RpmRefreshSec)):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// Verify common, usual packages that should be available in any distribution
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "packages/rpm",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				// Common value for a default installation
				"rpm" + fixture.OrKey + "SUSEConnect": map[string]interface{}{
					"architecture":    fixture.AnyValue,
					"id":              fixture.AnyValue,
					"installed_epoch": fixture.AnyValue,
					"release":         fixture.AnyValue,
					"version":         fixture.AnyValue,
				},
			},
		},
	})
}

// rpmBasedDistribution will return false for some distributions that have rpm installed but are not
// rpm baed distributions
func rpmBasedDistribution() (bool, error) {
	content, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		return false, err
	}
	regexps := []*regexp.Regexp{
		regexp.MustCompile("(?m)^NAME=\"Ubuntu\""),
		regexp.MustCompile("^NAME=\"Debian\""),
		regexp.MustCompile("^NAME=\"SLES\""),
	}
	for _, reg := range regexps {
		if reg.Match(content) {
			return false, nil
		}
	}
	return true, nil
}
