// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package license

import (
	"testing"

	"gotest.tools/assert"
)

const (
	basic     = "0123456789012345678901234567890123456789"
	euLicense = "eu01xx6789012345678901234567890123456789"
	jpLicense = "jp01xx6789012345678901234567890123456789"
)

func TestLicense_GetRegion(t *testing.T) {
	region := GetRegion(basic)
	assert.Equal(t, region, "")

	region = GetRegion(euLicense)
	assert.Equal(t, region, "eu")

	region = GetRegion(jpLicense)
	assert.Equal(t, region, "jp")
}

func TestLicense_IsRegionJP(t *testing.T) {
	t.Parallel()

	assert.Equal(t, IsRegionJP(jpLicense), true)
	assert.Equal(t, IsRegionJP(euLicense), false)
	assert.Equal(t, IsRegionJP(basic), false)
}
