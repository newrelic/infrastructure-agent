// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"golang.org/x/mod/semver"
	"os/exec"
	"regexp"
	"strings"
)

var versionRegex = regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)

// getAWSInstances return all the canary instance names mapped on ids.
func getAWSInstances(hostPrefix string) (map[string]string, error) {
	cmd := exec.Command("aws", []string{
		"ec2",
		"describe-instances",
		"--output", "text",
		"--filters", `Name=tag:Name,Values="` + hostPrefix + `*"`, `Name=instance-state-name,Values=running`,
		"--query", "Reservations[*].Instances[*].[Tags[?Key==`Name`], InstanceId]",
	}...)

	var out bytes.Buffer
	var outErr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &outErr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to get aws instances, error: %s", outErr.String())
	}

	return parseAWSInstances(out.String())
}

// parseAWSInstances parses the ec2 describe-instances response.
func parseAWSInstances(response string) (map[string]string, error) {
	result := make(map[string]string)
	split := strings.Split(response, "\n")

	for i := 0; i < len(split)-1; i += 2 {
		instanceID := strings.TrimSpace(split[i])

		if !strings.HasPrefix(split[i+1], "Name") {
			return nil, fmt.Errorf("unexpected line received for instanceName: %s", split[i+1])
		}
		instanceName := strings.TrimSpace(strings.TrimPrefix(split[i+1], "Name"))

		result[instanceID] = instanceName
	}

	return result, nil
}

// terminateInstances will terminate all instances.
func terminateInstances(idsToTerminate []string, instances map[string]string, dryRun bool) error {

	if len(idsToTerminate) == 0 {
		return nil
	}

	for _, id := range idsToTerminate {
		fmt.Println(colorizeRed(
			fmt.Sprintf("[DryRun: '%t'] Removing the following instance: %s -> %s",
				dryRun, id, instances[id])))

	}

	args := []string{
		"ec2",
		"terminate-instances",
		"--instance-ids",
	}
	args = append(args, idsToTerminate...)

	if !dryRun {
		cmd := exec.Command("aws", args...)

		var out bytes.Buffer
		var outErr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &outErr

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to terminate aws instances, error: %s", outErr.String())
		}

		fmt.Println(out.String())
	}

	return nil
}

func getPreviousCanaryVersion(instances map[string]string) (string, error) {
	versions, err := detectVersions(instances)
	if err != nil {
		return "", err
	}

	if len(versions) < 2 {
		return "", fmt.Errorf("in order to use it, you need to provision 2 versions of canaries first")
	}

	semver.Sort(versions)

	return versions[len(versions)-2], nil
}

// getInstancesToPrune will filter the instances that should be terminated.
func getInstancesToPrune(instances map[string]string) ([]string, error) {
	versions, err := detectVersions(instances)
	if err != nil {
		return nil, err
	}

	var idsToRemove []string

	for _, versionToRemove := range getVersionsToRemove(versions) {
		for id, name := range instances {
			if !strings.Contains(name, versionToRemove) {
				continue
			}
			idsToRemove = append(idsToRemove, id)
		}
	}

	return idsToRemove, nil
}

// getVersionsToRemove keeps the latest 2 version of infra agent.
func getVersionsToRemove(versions []string) []string {
	semver.Sort(versions)

	if len(versions) > 2 {
		return versions[:len(versions)-2]
	}

	return []string{}
}

// detectVersions will get the instance names, extract the agent version and return a sorted unique list.
func detectVersions(instances map[string]string) ([]string, error) {
	var versions []string

	unique := make(map[string]struct{})

	for _, name := range instances {
		matches := versionRegex.FindAllString(name, 1)
		if len(matches) < 1 {
			return nil, fmt.Errorf("failed to match version in instance name: '%s'", name)
		}

		version := matches[0]
		if _, exists := unique[version]; !exists {
			unique[version] = struct{}{}
			versions = append(versions, version)
		}
	}

	return versions, nil
}
