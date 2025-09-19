// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/counter"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/naming"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

const (
	defaultDockerAPIVersion = "1.24"
	metricAnnotationsToAdd  = 6
)

// Discoverer returns a Docker container discoverer from the provided configuration.
// The fetching process will return an array of map values for each discovered container, with the
// keys discovery.port and discovery.ip
func Discoverer(d discovery.Container) (fetchDiscoveries func() (discoveries []discovery.Discovery, err error), err error) {
	if d.ApiVersion == "" {
		d.ApiVersion = defaultDockerAPIVersion
	}
	matcher, err := discovery.NewMatcher(d.Match)
	if err != nil {
		return nil, err
	}
	return func() ([]discovery.Discovery, error) {
		return fetch(d, &matcher)
	}, nil
}

func fetch(d discovery.Container, matcher *discovery.FieldsMatcher) ([]discovery.Discovery, error) {
	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer dc.Close()

	containers, err := dc.ContainerList(context.Background(), container.ListOptions{})
	if err != nil {
		return nil, err
	}

	return getDiscoveries(containers, matcher), nil
}

// getDiscoveries will filter container list to only the ones that match the config and extract discovery variables from those.
func getDiscoveries(containers []container.Summary, matcher *discovery.FieldsMatcher) []discovery.Discovery {
	var matches []discovery.Discovery

	for _, cont := range containers {
		// discovery attributes that identify the container
		labels := map[string]string{}
		for k, v := range cont.Labels {
			labels[data.LabelInfix+k] = v
		}
		if len(cont.Names) > 0 {
			name := cont.Names[0]
			if name[0] == '/' {
				name = name[1:]
			}
			labels[data.Name] = name
		}
		labels[data.Image] = cont.Image

		labels[data.ContainerID] = cont.ID

		index := 0
		for _, network := range cont.NetworkSettings.Networks {
			if index == 0 {
				labels[data.PrivateIP] = network.IPAddress
			}
			labels[data.PrivateIP+"."+strconv.Itoa(index)] = network.IPAddress
			index++
		}

		addPorts(cont, labels)

		// only containers matching all the criteria will be added
		if matcher.All(labels) {
			prefixedLabels := discovery.LabelsToMap(data.DiscoveryPrefix, labels)

			ma := make(data.InterfaceMap, metricAnnotationsToAdd)
			naming.AddImage(ma, cont.Image)
			naming.AddImageID(ma, cont.ImageID)
			naming.AddContainerName(ma, strings.TrimPrefix(cont.Names[0], "/"))
			naming.AddContainerID(ma, cont.ID)
			naming.AddLabels(ma, cont.Labels)
			naming.AddCommand(ma, cont.Command)

			matches = append(matches, discovery.Discovery{
				Variables: prefixedLabels,
				EntityRewrites: []data.EntityRewrite{
					{
						Action:       data.EntityRewriteActionReplace,
						Match:        naming.ToVariable(data.IP),
						ReplaceField: data.ContainerReplaceFieldPrefix + naming.ToVariable(data.ContainerID),
					},
				},
				MetricAnnotations: ma,
			})
		}
	}
	return matches
}

func addPorts(cont container.Summary, labels map[string]string) {
	// sort ports from lower to higher so we are always consistent with the returned ports
	sort.Slice(cont.Ports, func(i, j int) bool {
		return cont.Ports[i].PrivatePort < cont.Ports[j].PrivatePort
	})

	types := counter.ByKind{}
	firstPublic := true
	firstPrivate := true

	for index, port := range cont.Ports {
		indexStr := "." + strconv.Itoa(index)
		labels[data.IP+indexStr] = port.IP
		tIdx := types.Count(port.Type)

		publicPort := strconv.Itoa(int(port.PublicPort))
		privatePort := strconv.Itoa(int(port.PrivatePort))

		if firstPublic && port.PublicPort > 0 && isIPv4(port.IP) {
			labels[data.IP] = port.IP
			labels[data.Port] = publicPort
			firstPublic = false
		}

		labels[data.Ports+indexStr] = publicPort

		if firstPrivate {
			labels[data.PrivatePort] = privatePort
			firstPrivate = false
		}

		labels[data.PrivatePorts+indexStr] = privatePort

		// label ports by type (e.g. discovery.port.tcp.1)
		if port.Type != "" {
			if tIdx == 0 {
				labels[data.Ports+"."+port.Type] = publicPort
				labels[data.PrivatePorts+"."+port.Type] = privatePort
			}
			labels[data.Ports+"."+port.Type+indexStr] = publicPort
			labels[data.PrivatePorts+"."+port.Type+indexStr] = privatePort
		}
	}
}

// isIPv4 returns true if ip string has a IPv4 format.
func isIPv4(ip string) bool {
	return net.ParseIP(ip).To4() != nil
}
