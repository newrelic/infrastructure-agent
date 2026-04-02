// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package license

import (
	"gotest.tools/assert"
	"testing"
)

const (
	basic = "0123456789012345678901234567890123456789"
	eu    = "eu01xx6789012345678901234567890123456789"
	jp    = "jp01xx6789012345678901234567890123456789"
)

func TestLicense_GetRegion(t *testing.T) {
	region := GetRegion(basic)
	assert.Equal(t, region, "")

	region = GetRegion(eu)
	assert.Equal(t, region, "eu")

	region = GetRegion(jp)
	assert.Equal(t, region, "jp")
}

func TestLicense_IsRegionJP(t *testing.T) {
	assert.Equal(t, IsRegionJP(jp), true)
	assert.Equal(t, IsRegionJP(eu), false)
	assert.Equal(t, IsRegionJP(basic), false)
}
