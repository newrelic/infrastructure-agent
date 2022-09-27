// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDetectVersions_Valid(t *testing.T) {
	instances := map[string]string{
		"i-042ab6db828c1c318": "canary:v1.2.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c319": "canary:v1.2.3:arm64:ubuntu20.04",
		"i-042ab6db828c1c314": "canary:v0.1.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c315": "canary:v0.1.3:arm64:ubuntu20.04",
		"i-042ab6db828c1c316": "canary:v1.1.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c317": "canary:v1.1.3:arm64:ubuntu20.04",
	}

	expected := []string{
		"v1.2.3",
		"v0.1.3",
		"v1.1.3",
	}

	actual, err := detectVersions(instances)
	assert.NoError(t, err)

	assert.ElementsMatch(t, expected, actual)
}

func TestDetectVersions_Invalid(t *testing.T) {
	instances := map[string]string{
		"i-042ab6db828c1c314": "canary:v0.1.a3:amd64:ubuntu20.04",
		"i-042ab6db828c1c315": "canary:0.1.3:arm64:ubuntu20.04",
	}

	actual, err := detectVersions(instances)
	assert.Error(t, err)

	var expected []string
	assert.Equal(t, expected, actual)
}

func TestGetVersionsToRemove(t *testing.T) {
	actual := getVersionsToRemove([]string{
		"v3.3.3",
		"v2.2.2",
		"v1.1.1",
	})

	expected := []string{
		"v1.1.1",
	}

	assert.Equal(t, expected, actual)

	actual = getVersionsToRemove([]string{
		"v1.1.1",
		"v2.2.2",
	})

	expected = []string{}
	assert.Equal(t, expected, actual)
}

func TestGetInstancesToPrune(t *testing.T) {
	instances := map[string]string{
		"i-042ab6db828c1c318": "canary:v1.2.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c319": "canary:v1.2.3:arm64:ubuntu20.04",
		"i-042ab6db828c1c314": "canary:v0.1.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c315": "canary:v0.1.3:arm64:ubuntu20.04",
		"i-042ab6db828c1c316": "canary:v1.1.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c317": "canary:v1.1.3:arm64:ubuntu20.04",
	}

	expected := []string{
		"i-042ab6db828c1c314",
		"i-042ab6db828c1c315",
	}

	actual, err := getInstancesToPrune(instances)
	assert.NoError(t, err)

	assert.ElementsMatch(t, expected, actual)
}

func TestGetPreviousCanaryVersion(t *testing.T) {
	instances := map[string]string{
		"i-042ab6db828c1c318": "canary:v1.2.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c319": "canary:v1.2.3:arm64:ubuntu20.04",
		"i-042ab6db828c1c314": "canary:v0.1.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c315": "canary:v0.1.3:arm64:ubuntu20.04",
		"i-042ab6db828c1c316": "canary:v1.1.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c317": "canary:v1.1.3:arm64:ubuntu20.04",
	}

	expected := "v1.1.3"

	actual, err := getPreviousCanaryVersion(instances)
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)

	instances = map[string]string{
		"i-042ab6db828c1c318": "canary:v1.2.3:amd64:ubuntu20.04",
		"i-042ab6db828c1c319": "canary:v1.2.3:arm64:ubuntu20.04",
	}

	expected = ""

	actual, err = getPreviousCanaryVersion(instances)
	assert.Error(t, err)

	assert.Equal(t, expected, actual)
}

func TestParseAWSInstancesValid(t *testing.T) {
	awsResponse := `i-042ab6db828c1c318
Name	canary:v1.2.3:amd64:ubuntu20.04
i-055adb7d536497654
Name	canary:v1.2.4:amd64:ubuntu20.04
`
	expected := map[string]string{
		"i-042ab6db828c1c318": "canary:v1.2.3:amd64:ubuntu20.04",
		"i-055adb7d536497654": "canary:v1.2.4:amd64:ubuntu20.04",
	}

	actual, err := parseAWSInstances(awsResponse)

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestParseAWSInstancesInvalid(t *testing.T) {
	awsResponse := `i-042ab6db828c1c318
canary:v1.2.3:amd64:ubuntu20.04
i-055adb7d536497654
Name	canary:v1.2.4:amd64:ubuntu20.04
`
	var expected map[string]string

	actual, err := parseAWSInstances(awsResponse)

	assert.Error(t, err)
	assert.Equal(t, expected, actual)
}
