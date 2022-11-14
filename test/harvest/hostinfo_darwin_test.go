// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin && harvest
// +build darwin,harvest

package harvest

import (
	"github.com/newrelic/infrastructure-agent/internal/plugins/darwin"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

func TestHostInfoDarwin(t *testing.T) {
	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.RunMode = config.ModeUnprivileged
	})
	log.SetLevel(logrus.DebugLevel)
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	a.RegisterPlugin(darwin.NewHostinfoPlugin(a.Context, common.NewHostInfoCommon("test", true, cloudDetector)))
	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	reqCopy := req
	fixture.AssertRequestContainsInventoryDeltas(t, reqCopy, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/system",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"system": map[string]interface{}{
					"id":               "system",
					"distro":           fixture.AnyValue,
					"kernel_version":   fixture.AnyValue,
					"host_type":        fixture.AnyValue,
					"cpu_num":          fixture.AnyValue,
					"total_cpu":        fixture.AnyValue,
					"ram":              fixture.AnyValue,
					"boot_timestamp":   fixture.AnyValue,
					"agent_version":    fixture.AnyValue,
					"agent_name":       "Infrastructure",
					"operating_system": "macOS",
					"product_uuid":     fixture.AnyValue,
					"agent_mode":       "unprivileged",
				},
			},
		},
	})
}
