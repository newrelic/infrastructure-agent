// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/counter"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/naming"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
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
	var matches []discovery.Discovery

	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer dc.Close()

	containers, err := dc.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	for _, cont := range containers {
		// discovery attributes that identify the container
		labels := map[string]string{}
		for k, v := range cont.Labels {
			labels[naming.LabelInfix+k] = v
		}
		if len(cont.Names) > 0 {
			name := cont.Names[0]
			if name[0] == '/' {
				name = name[1:]
			}
			labels[naming.Name] = name
		}
		labels[naming.Image] = cont.Image

		labels[naming.ContainerID] = cont.ID

		index := 0
		for _, network := range cont.NetworkSettings.Networks {
			if index == 0 {
				labels[naming.PrivateIP] = network.IPAddress
			}
			labels[naming.PrivateIP+"."+strconv.Itoa(index)] = network.IPAddress
			index++
		}

		addPorts(cont, labels)

		// only containers matching all the criteria will be added
		if matcher.All(labels) {
			prefixedLabels := discovery.LabelsToMap(naming.DiscoveryPrefix, labels)

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
						Action:       naming.EntityRewriteActionReplace,
						Match:        naming.ToVariable(naming.IP),
						ReplaceField: naming.ContainerReplaceFieldPrefix + naming.ToVariable(naming.ContainerID),
					},
				},
				MetricAnnotations: ma,
			})
		}
	}

	return matches, nil
}

func addPorts(cont types.Container, labels map[string]string) {
	// sort ports from lower to higher so we are always consistent with the returned ports
	sort.Slice(cont.Ports, func(i, j int) bool {
		return cont.Ports[i].PrivatePort < cont.Ports[j].PrivatePort
	})

	types := counter.ByKind{}
	firstPublic := true
	firstPrivate := true

	for index, port := range cont.Ports {
		indexStr := "." + strconv.Itoa(index)
		labels[naming.IP+indexStr] = port.IP
		labels[naming.IP] = port.IP

		tIdx := types.Count(port.Type)
		publicPort := strconv.Itoa(int(port.PublicPort))
		privatePort := strconv.Itoa(int(port.PrivatePort))
		if firstPublic {
			labels[naming.Port] = publicPort
			firstPublic = false
		}
		labels[naming.Ports+indexStr] = publicPort
		if firstPrivate {
			labels[naming.PrivatePort] = privatePort
			firstPrivate = false
		}
		labels[naming.PrivatePorts+indexStr] = privatePort

		// label ports by type (e.g. discovery.port.tcp.1)
		if port.Type != "" {
			if tIdx == 0 {
				labels[naming.Ports+"."+port.Type] = publicPort
				labels[naming.PrivatePorts+"."+port.Type] = privatePort
			}
			labels[naming.Ports+"."+port.Type+indexStr] = publicPort
			labels[naming.PrivatePorts+"."+port.Type+indexStr] = privatePort
		}
	}
}
