// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMatchingContainers(t *testing.T) {
	givenContainerList := []container.Summary{
		{
			ID: "484c2678906bed94a51fe12ec1fc8ac55f177453dba00c1b0ae0a22f4b655e41",
			Names: []string{
				"/sharp_beaver",
			},
			Image:   "test-server",
			ImageID: "sha256:8282995409c26b82d66243f169f6695115a06ce860966db549f8ca09dcbb9767",
			Command: "/bin/bash -c 'java ${JAVA_OPTS} -jar /test-server.jar'",
			Created: 1653916418,
			Ports: []container.Port{
				{
					IP:          "0.0.0.0",
					PrivatePort: 7199,
					PublicPort:  7199,
					Type:        "tcp",
				},
				{
					IP:          "",
					PrivatePort: 4567,
					PublicPort:  0,
					Type:        "tcp",
				},
			},
			SizeRw:     0,
			SizeRootFs: 0,
			Labels:     map[string]string{},
			State:      "running",
			Status:     "Up 8 minutes",
			HostConfig: struct {
				NetworkMode string            "json:\",omitempty\"" //nolint:tagalign
				Annotations map[string]string "json:\",omitempty\"" //nolint:tagalign
			}{
				NetworkMode: "default",
			},
			NetworkSettings: &container.NetworkSettingsSummary{
				Networks: map[string]*network.EndpointSettings{
					"bridge": {
						IPAMConfig:          (*network.EndpointIPAMConfig)(nil),
						Links:               []string(nil),
						Aliases:             []string(nil),
						NetworkID:           "e0effada2c3eab26fb73188d0193952ef9a3c985e64cabd76c95196000405418",
						EndpointID:          "8e143f019bb8549a5594a31058c077eb774960b9b825f819f9593ab57a591f71",
						Gateway:             "172.17.0.1",
						IPAddress:           "172.17.0.2",
						IPPrefixLen:         16,
						IPv6Gateway:         "",
						GlobalIPv6Address:   "",
						GlobalIPv6PrefixLen: 0,
						MacAddress:          "02:42:ac:11:00:02",
						DriverOpts:          map[string]string(nil),
					},
				},
			},

			Mounts: []container.MountPoint{},
		},
	}

	expectedDiscoveryData := []discovery.Discovery{
		{
			Variables: data.Map{
				"discovery.containerId":         "484c2678906bed94a51fe12ec1fc8ac55f177453dba00c1b0ae0a22f4b655e41",
				"discovery.image":               "test-server",
				"discovery.ip":                  "0.0.0.0",
				"discovery.ip.0":                "",
				"discovery.ip.1":                "0.0.0.0",
				"discovery.name":                "sharp_beaver",
				"discovery.port":                "7199",
				"discovery.ports.0":             "0",
				"discovery.ports.1":             "7199",
				"discovery.ports.tcp":           "0",
				"discovery.ports.tcp.0":         "0",
				"discovery.ports.tcp.1":         "7199",
				"discovery.private.ip":          "172.17.0.2",
				"discovery.private.ip.0":        "172.17.0.2",
				"discovery.private.port":        "4567",
				"discovery.private.ports.0":     "4567",
				"discovery.private.ports.1":     "7199",
				"discovery.private.ports.tcp":   "4567",
				"discovery.private.ports.tcp.0": "4567",
				"discovery.private.ports.tcp.1": "7199",
			},
			MetricAnnotations: data.InterfaceMap{
				"command":       "/bin/bash -c 'java ${JAVA_OPTS} -jar /test-server.jar'",
				"containerId":   "484c2678906bed94a51fe12ec1fc8ac55f177453dba00c1b0ae0a22f4b655e41",
				"containerName": "sharp_beaver",
				"image":         "test-server",
				"imageId":       "sha256:8282995409c26b82d66243f169f6695115a06ce860966db549f8ca09dcbb9767",
				"label":         map[string]string{}},
			EntityRewrites: []data.EntityRewrite{
				{
					Action:       "replace",
					Match:        "${ip}",
					ReplaceField: "container:${containerId}",
				},
			},
		},
	}

	matcher, err := discovery.NewMatcher(map[string]string{
		"image": "/test-server/",
	})
	require.NoError(t, err)

	actualDiscoveryData := getDiscoveries(givenContainerList, &matcher)
	assert.Equal(t, expectedDiscoveryData, actualDiscoveryData)
}

func TestIsIPv4(t *testing.T) {
	testCases := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "ValidIpV4",
			ip:       "127.0.0.1",
			expected: true,
		},
		{
			name:     "EmptyIp",
			ip:       "",
			expected: false,
		},
		{
			name:     "IpV6",
			ip:       "0000:0000:0000:0000:0000:0000:0000:0001",
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, isIPv4(testCase.ip))
		})
	}
}
