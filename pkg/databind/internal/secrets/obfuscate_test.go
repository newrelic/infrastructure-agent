// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"encoding/json"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDecodeString(t *testing.T) {
	expectedDecrypted := "test"

	obfuscateStruct := Obfuscated{
		Key:    "secretPass",
		Secret: "BwAQBg==", // => decrypts as test
	}

	g := ObfuscateGatherer(&obfuscateStruct)
	r, err := g()
	if err != nil {
		t.Errorf("obfuscate failed: %v ", err)
	}

	unboxed, ok := r.(string)
	if !ok {
		t.Error("Unable to unbox result")
	}

	assert.Equal(t, expectedDecrypted, unboxed)
}

func TestDecodeJSON(t *testing.T) {
	expectedDecrypted := "{\"pass1\":\"test1\",\"pass2\":\"test2\"}"

	obfuscateStruct := Obfuscated{
		Key:    "secretPass",
		Secret: "CEcTExYHYUNJUQcAEAZUVnxDAxIAFlFQX1YkBAAHQUce", // => decrypts as test
	}

	g := ObfuscateGatherer(&obfuscateStruct)
	r, err := g()
	if err != nil {
		t.Errorf("obfuscate failed: %v ", err)
	}

	unboxed, ok := r.(data.InterfaceMap)
	if !ok {
		t.Error("Unable to unbox result")
	}

	if unboxed == nil {
		t.Errorf("Result is nil")
	}
	result, err := json.Marshal(unboxed)
	assert.NoError(t, err)
	assert.Equal(t, expectedDecrypted, string(result))
}
