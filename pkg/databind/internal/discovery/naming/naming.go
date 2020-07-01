// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"regexp"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

const (
	DiscoveryPrefix             = "discovery."
	LabelInfix                  = "label."
	ContainerReplaceFieldPrefix = "container:"

	Port                       = "port"
	Ports                      = "ports"
	IP                         = "ip"
	PrivatePort                = "private.port"
	PrivatePorts               = "private.ports"
	PrivateIP                  = "private.ip"
	Name                       = "name"
	Image                      = "image"
	ImageID                    = "imageId"
	ContainerID                = "containerId"
	ContainerName              = "containerName"
	Label                      = "label"
	Command                    = "command"
	DockerContainerName        = "dockerContainerName"
	EntityRewriteActionReplace = "replace"

	// placeholderRegex matches anything that is "${something}".
	// It isn't a "greedy" regex as it will stop at the first closing brace.
	// This means if you have a string "a string with ${one} and ${two} place holders" it will
	// return you "${one}" and "${two}" and not "${one} and ${two}".
	placeholderRegex  = "\\$\\{.+?[}]"
	matchStartIdx     = 0
	matchOpenBraceIdx = 2
	oneMatch          = 1
)

// ToVariable converts a string into a discovery variable.
func ToVariable(name string) string {
	return "${" + name + "}"
}

// AddPrefixToVariable is used to inject a prefix into a discovery variable name.
func AddPrefixToVariable(prefix, variable string) string {

	if prefix == "" {
		return variable
	}

	if prefix[len(prefix)-1] != '.' {
		prefix += "."
	}

	r, _ := regexp.Compile(placeholderRegex)
	match := r.FindAllString(variable, -1)
	for i := range match {
		if !strings.HasPrefix(match[i], "${"+prefix) { // check string doesn't already have prefix
			variable = strings.Replace(variable, match[i],
				match[i][matchStartIdx:matchOpenBraceIdx]+
					prefix+
					match[i][matchOpenBraceIdx:],
				oneMatch)
		}
	}

	return variable
}

// AddImage adds Image name to metricAnnotations
func AddImage(metricAnnotations data.InterfaceMap, image string) {
	metricAnnotations[Image] = image
}

// AddImageID adds Image ID to metricAnnotations
func AddImageID(metricAnnotations data.InterfaceMap, imageID string) {
	metricAnnotations[ImageID] = imageID
}

// AddContainerName adds container name to metricAnnotations
func AddContainerName(metricAnnotations data.InterfaceMap, containerName string) {
	metricAnnotations[ContainerName] = containerName
}

// AddContainerID adds container ID to metricAnnotations
func AddContainerID(metricAnnotations data.InterfaceMap, containerID string) {
	metricAnnotations[ContainerID] = containerID
}

// AddLabels adds Docker labels to metricAnnotations
func AddLabels(metricAnnotations data.InterfaceMap, labels map[string]string) {
	metricAnnotations[Label] = labels
}

// AddCommand adds docker command to metricAnnotations
func AddCommand(metricAnnotations data.InterfaceMap, command string) {
	metricAnnotations[Command] = command
}

// AddDockerContainerName adds docker container name to metricAnnotations
func AddDockerContainerName(metricAnnotations data.InterfaceMap, dockerContainerName string) {
	metricAnnotations[DockerContainerName] = dockerContainerName
}
