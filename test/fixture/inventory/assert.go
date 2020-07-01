// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Any string
type ContainsString string
type Nil string

const AnyValue = Any("any-value-is-ok")
const NilValue = Nil("this-field-is-not-present")

// AssertRequestContainsInventoryDeltas checks, for each RawDelta entry, that is is contained in the
// request as a subset of all the entries.
func AssertRequestContainsInventoryDeltas(t *testing.T, req http.Request, expected []*inventoryapi.RawDelta) {
	reqBytes, err := ioutil.ReadAll(req.Body)
	assert.NoError(t, err)

	//t.Logf("bytes::: %s", reqBytes)

	sent := inventoryapi.PostDeltaBody{}
	assert.NoError(t, json.Unmarshal(reqBytes, &sent))

	sentDeltas := asMap(sent)

	// For each expected RawDelta entry
	for _, expectedDelta := range expected {
		// Check the expected delta has been sent
		assert.Contains(t, sentDeltas, expectedDelta.Source)
		sentDelta := sentDeltas[expectedDelta.Source]
		if sentDelta == nil { // Stop testing something that does not exist (test already failed)
			continue
		}

		// Compares metadata
		assert.Equal(t, expectedDelta.FullDiff, sentDelta.FullDiff, "Comparing FullDiff for %q", expectedDelta.Source)
		assert.Equal(t, expectedDelta.ID, sentDelta.ID, "Comparing ID for %q", expectedDelta.Source)

		assertSubTreeContained(t, expectedDelta.Diff, sentDelta.Diff)

		// timestamp assertion is only to check that it's greater than current time
		assert.True(t, time.Now().After(time.Unix(sentDelta.Timestamp, 0)), "inventory ts not lower than current")
	}
}

// If expected and actual are maps: checks that all the entries in the expected are contained in the actual
// Otherwise, compare whether they are equal
func assertSubTreeContained(t *testing.T, expected interface{}, actual interface{}) {
	// If an expected diff entry is nil, it just checks it exist (does not compare contents)
	switch expected.(type) {
	case Any:
		return
	case ContainsString:
		actualStr, ok := actual.(string)
		require.True(t, ok, "expected string value but got: %+v", actual)
		assert.True(t, strings.Contains(actualStr, string(expected.(ContainsString))),
			"actual string: %s, does not contain: %s", actualStr, expected)
		return
	case Nil:
		assert.Nil(t, actual)
		return
	}

	if expectedField, ok := expected.(config.CustomAttributeMap); ok {
		if actualField, ok := actual.(config.CustomAttributeMap); ok {
			for innerFieldName, innerField := range expectedField {
				// Expected Nil fields are not being asserted to be contained
				if _, ok := innerField.(Nil); !ok {
					assert.Contains(t, actualField, innerFieldName)
				}
				assertSubTreeContained(t, innerField, actualField[innerFieldName])
			}
			return
		}
		if actualField, ok := actual.(map[string]interface{}); ok {
			for innerFieldName, innerField := range expectedField {
				// Expected Nil fields are not being asserted to be contained
				if _, ok := innerField.(Nil); !ok {
					assert.Contains(t, actualField, innerFieldName)
				}
				assertSubTreeContained(t, innerField, actualField[innerFieldName])
			}
			return
		}
	}

	if expectedField, ok := expected.(map[string]interface{}); ok {
		if actualField, ok := actual.(map[string]interface{}); ok {
			for innerFieldName, innerField := range expectedField {
				// Expected Nil fields are not being asserted to be contained
				if _, ok := innerField.(Nil); !ok {
					assert.Contains(t, actualField, innerFieldName)
				}
				assertSubTreeContained(t, innerField, actualField[innerFieldName])
			}
			return
		}
	}
	assert.Equal(t, expected, actual)
}

func asMap(postDeltas inventoryapi.PostDeltaBody) map[string]*inventoryapi.RawDelta {

	deltasMap := map[string]*inventoryapi.RawDelta{}
	for _, delta := range postDeltas.Deltas {
		deltasMap[delta.Source] = delta
	}
	return deltasMap
}
