// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common //nolint:revive // package name predates this file; not introducing a rename here

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlattenLabels(t *testing.T) {
	t.Parallel()

	type sample struct {
		Name string `json:"name"`
	}

	t.Run("no tags leaves the payload untouched", func(t *testing.T) {
		t.Parallel()

		data, err := FlattenLabels(sample{Name: "host1"}, nil)
		require.NoError(t, err)

		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &got))
		require.Equal(t, map[string]interface{}{"name": "host1"}, got)
	})

	t.Run("tags are merged as top-level label.<key> attributes", func(t *testing.T) {
		t.Parallel()

		data, err := FlattenLabels(sample{Name: "host1"}, map[string]string{"env": "prod"})
		require.NoError(t, err)

		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &got))
		require.Equal(t, map[string]interface{}{"name": "host1", "label.env": "prod"}, got)
	})
}
