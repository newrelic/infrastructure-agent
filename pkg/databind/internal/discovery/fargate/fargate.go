// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fargate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/counter"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/naming"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

const (
	VM_META_DATA_URL       = "http://169.254.170.2/v2/metadata"
	metricAnnotationsToAdd = 6
)

// Discoverer returns a Fargate discoverer from the provided container discovery configuration.
// The fetching process will return an array of map values for each discovered container, with the
// keys discovery.port and discovery.ip
func Discoverer(d discovery.Container) (fetchDiscoveries func() (discoveries []discovery.Discovery, err error), err error) {
	matcher, err := discovery.NewMatcher(d.Match)
	if err != nil {
		return nil, err
	}
	return func() ([]discovery.Discovery, error) {
		meta, err := awsMetadata()
		if err != nil {
			return nil, err
		}
		return match(meta, &matcher)
	}, nil
}

func awsMetadata() (*TaskMetadata, error) {
	client := &http.Client{}
	resp, err := client.Get(VM_META_DATA_URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server responded %v - %v", resp.StatusCode, resp.Status)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	meta := TaskMetadata{}
	if err := json.Unmarshal(bodyBytes, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func match(meta *TaskMetadata, matcher *discovery.FieldsMatcher) ([]discovery.Discovery, error) {
	var matches []discovery.Discovery

	for _, cont := range meta.Containers {
		// labels to identify the container
		labels := map[string]string{}
		for k, v := range cont.Labels {
			labels[naming.LabelInfix+k] = v
		}
		labels[naming.Name] = cont.Name
		labels[naming.Image] = cont.Image
		labels[naming.ContainerID] = cont.DockerID

		addPorts(cont, labels)

		index := 0
		for _, network := range cont.Networks {
			for _, address := range network.IPv4Addresses {
				if index == 0 {
					labels[naming.IP] = address // at the moment, fargate ips are also private ips
					labels[naming.PrivateIP] = address
				}
				indexStr := "." + strconv.Itoa(index)
				labels[naming.IP+indexStr] = address // at the moment, fargate ips are also private ips
				labels[naming.PrivateIP+indexStr] = address
				index++
			}
		}
		// only containers matching all the criteria will be added
		if matcher.All(labels) {
			containerLabels := discovery.LabelsToMap(naming.DiscoveryPrefix, labels)

			ma := make(data.InterfaceMap, metricAnnotationsToAdd)
			naming.AddImage(ma, cont.Image)
			naming.AddImageID(ma, cont.ImageID)
			naming.AddContainerName(ma, cont.Name)
			naming.AddContainerID(ma, cont.DockerID)
			naming.AddLabels(ma, cont.Labels)
			naming.AddDockerContainerName(ma, cont.DockerName)

			matches = append(matches, discovery.Discovery{
				Variables: containerLabels,
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

func addPorts(cont awsContainer, labels map[string]string) {
	// sort ports from lower to higher so we are always consistent with the returned ports
	sort.Slice(cont.Ports, func(i, j int) bool {
		return cont.Ports[i].ContainerPort < cont.Ports[j].ContainerPort
	})

	protocols := counter.ByKind{}
	firstPublic := true
	firstPrivate := true
	for index, p := range cont.Ports {
		pNum := protocols.Count(p.Protocol)

		// keeps the protocol type to allow referencing as part of the path
		if p.HostPort != 0 {
			portStr := strconv.Itoa(int(p.HostPort))
			if firstPublic {
				labels[naming.Port] = portStr // discovery.port = <...>
				firstPublic = false
			}
			labels[naming.Ports+"."+strconv.Itoa(index)] = portStr // discovery.port.0 = <...>

			if p.Protocol != "" {
				if pNum == 0 {
					labels[naming.Ports+"."+p.Protocol] = portStr // discovery.port.tcp = <...>
				}
				labels[naming.Ports+"."+p.Protocol+"."+strconv.Itoa(pNum)] = portStr // discovery.port.tcp.0 = <...>
			}
		}
		if p.ContainerPort != 0 {
			portStr := strconv.Itoa(int(p.ContainerPort))
			if firstPrivate {
				labels[naming.PrivatePort] = portStr // discovery.private.port = <...>
				firstPrivate = false
			}
			labels[naming.PrivatePorts+"."+strconv.Itoa(index)] = portStr // discovery.private.port.0 = <...>
			if p.Protocol != "" {
				if pNum == 0 {
					labels[naming.PrivatePorts+"."+p.Protocol] = portStr // discovery.private.port.tcp = <...>
				}
				labels[naming.PrivatePorts+"."+p.Protocol+"."+strconv.Itoa(pNum)] = portStr // discovery.private.port.tcp.0 = <...>
			}
		}
	}
}
