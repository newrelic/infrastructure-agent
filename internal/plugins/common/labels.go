// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
)

// labelPrefix is prepended to each dynamic tag key when flattened into inventory attributes.
const labelPrefix = "label."

// FlattenLabels marshals v, then merges tags into the result as top-level "label.<key>"
// attributes. v must not itself implement json.Marshaler in a way that would recurse into
// this function - callers typically pass a type-aliased copy of the struct calling this from
// its own MarshalJSON to avoid infinite recursion.
func FlattenLabels(v any, tags map[string]string) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value for label flattening: %w", err)
	}

	if len(tags) == 0 {
		return data, nil
	}

	var merged map[string]interface{}
	if err := json.Unmarshal(data, &merged); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value for label flattening: %w", err)
	}

	for key, value := range tags {
		merged[labelPrefix+key] = value
	}

	flattened, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flattened labels: %w", err)
	}

	return flattened, nil
}
