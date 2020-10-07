package license

import (
	"gotest.tools/assert"
	"testing"
)

const (
	basic   = "0123456789012345678901234567890123456789"
	eu      = "eu01xx6789012345678901234567890123456789"
	federal = "gov01x6789012345678901234567890123456789"
)

func TestLicense_IsFederalCompliance(t *testing.T) {
	isFed := IsFederalCompliance(basic)
	assert.Equal(t, isFed, false)

	isFed = IsFederalCompliance(eu)
	assert.Equal(t, isFed, false)

	isFed = IsFederalCompliance(federal)
	assert.Equal(t, isFed, true)
}

func TestLicense_GetRegion(t *testing.T) {
	region := GetRegion(basic)
	assert.Equal(t, region, "")

	region = GetRegion(eu)
	assert.Equal(t, region, "eu")

	region = GetRegion(federal)
	assert.Equal(t, region, "gov")
}
