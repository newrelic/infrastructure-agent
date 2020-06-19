// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"strings"
)

// Dummy integration reporting protocol v4.

func main() {
	fmt.Println(strings.Replace(`
{
  "protocol_version": "4",
  "integration": {
    "name": "Foo",
    "version": "1.0.0"
  },
  "data": [
    {
      "common": {
        "timestamp": 1586357933,
        "interval.ms": 10000,
        "attributes": {
          "host.name": "host-foo",
          "host.user": "foo-man-choo"
        }
      },
      "metrics": [
        {
          "name": "a.gauge",
          "type": "gauge",
          "value": 13,
          "attributes": {
            "key1": "val1"
          }
        },
        {
          "name": "a.summary",
          "type": "summary",
          "value": {
            "count": 10,
            "sum": 664,
            "min": 15,
            "max": 248
          }
        },
        {
          "name": "a.count",
          "type": "count",
          "value": 666
        }
      ],
      "entity": {
        "name": "a.entity.name",
        "type": "ASample",
        "displayName": "A display name",
        "tags": {
          "env": "testing"
        }
      },
      "inventory": {
        "foo": {
          "value": "bar"
        }
      },
      "events": []
    }
  ]
}
`, "\n", "", -1))
}
