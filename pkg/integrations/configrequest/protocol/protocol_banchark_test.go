// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"
)

var data = []byte(`
{
	"config_protocol_version": "1",
	"action": "register_config",
	"config_name": "myconfig",
	"config": {
		"variables": {},
		"integrations": [
			{
				"name": "nri-mysql",
				"interval": "15s"
			}
		]
	}
}
`)

func BenchmarkGetConfigProtocolBuilderValidRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		assert.NotNil(b, GetConfigProtocolBuilder(data))
	}
}

func BenchmarkGetConfigProtocolBuilderBigRequestNotProtocolConfig(b *testing.B) {
	content, err := ioutil.ReadFile(filepath.Join("testdata", "vsphere.json"))
	if err != nil {
		log.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		assert.Nil(b, GetConfigProtocolBuilder(content))
	}
}
