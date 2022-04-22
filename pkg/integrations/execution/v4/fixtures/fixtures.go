// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixtures

import (
	"github.com/newrelic/infrastructure-agent/pkg/integrations/execution/v4/testhelp"
	"path"
	"runtime"
	"strings"
)

var (
	BasicCmdWithSpace  = testhelp.Script(path.Join("..", "fixtures", "basic cmd"+getExtension()))
	TimestampDiscovery = testhelp.WrapScriptPath("..", "fixtures", "discoverer", "discoverer.go")
	// The following test can't use `testhelp.WrapScriptPath` as it has arguments passed to it
	InventoryGoFile = testhelp.Script(path.Join("..", "fixtures", "inventory", "inventory.go"))
)

func getExtension() string {
	if runtime.GOOS == "windows" {
		return ".ps1"
	}
	return ".sh"
}

// Payloads
const (
	Foo = `{
  "protocol_version": "4",
  "integration": {
    "name": "com.newrelic.foo",
    "version": "0.1.0"
  },
  "data": [
    {
      "common": {
        "timestamp": 1531414060739,
        "interval.ms": 10000,
        "attributes": {}
      },
      "metrics": [
        {
          "name": "foo.metric1",
          "type": "count",
          "value": 93,
          "attributes": {}
        }
      ],
      "entity": {
        "name": "unique foo",
        "type": "Foo",
        "metadata": {}
      },
      "inventory": {
        "inventory_foo": {
          "value": "bar"
        }
      },
      "events": []
    }
  ]
}`
)

// Payloads
var (
	FooBytes = []byte(strings.Replace(Foo, "\n", "", -1) + "\n")
)

// Integrations
var (
	SimpleGoFile        = testhelp.WrapScriptPath("fixtures", "simple", "simple.go")
	EnvironmentGoFile   = testhelp.WrapScriptPath("fixtures", "environment", "environment_verbose.go")
	ProtocolV4GoFile    = testhelp.WrapScriptPath("fixtures", "protocol_v4", "protocol_v4.go")
	ValidYAMLGoFile     = testhelp.WrapScriptPath("fixtures", "validyaml", "validyaml.go")
	LongTimeGoFile      = testhelp.WrapScriptPath("fixtures", "longtime", "longtime.go")
	LongRunningHBGoFile = testhelp.WrapScriptPath("fixtures", "longrunning_hb", "longrunning_hb.go")
	HugeGoFile          = testhelp.WrapScriptPath("fixtures", "huge", "huge.go")
	CmdReqGoFile        = testhelp.WrapScriptPath("fixtures", "cmdreq", "cmdreq.go")
	CfgReqGoFile        = testhelp.WrapScriptPath("fixtures", "cfgreq", "cfgreq.go")
)
